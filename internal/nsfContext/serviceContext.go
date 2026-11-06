package nsfContext

import (
	"encoding/json"
	"fmt"
	"nsf/internal/db"
	"nsf/logger"
	"slices"
	"strings"
	"sync"
)

type serviceContextIE struct {
	db.DbIf

	*logger.NsfLogger
}

type serviceContext struct {
	db.DbIf
	mu sync.RWMutex

	*logger.NsfLogger
}

func newServiceContext(serviceContextIE *serviceContextIE) *serviceContext {
	return &serviceContext{
		DbIf: serviceContextIE.DbIf,

		NsfLogger: serviceContextIE.NsfLogger,
	}
}

func (s *serviceContext) release() {
	s.NsfLogger.SvcLog.Infoln("Release serviceContext...")

	s.NsfLogger.SvcLog.Infoln("serviceContext released")
}

// Each NF is one database record, so partial deregistration can be saved atomically.
// Discovery reads the database rather than an ephemeral index, including after restart.
const nfKeyPrefix = "nf:"

type registeredNF struct {
	NfName     string   `json:"nf_name"`
	SbiAddress string   `json:"sbi_address"`
	SbiPort    int      `json:"sbi_port"`
	Services   []string `json:"services"`
}

func (s *serviceContext) registerServices(nfName, sbiAddr string, sbiPort int, services []string) error {
	s.NsfLogger.SvcLog.Tracef("Registering %s services", nfName)
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(services) == 0 {
		return nil
	}
	nf := registeredNF{NfName: nfName, SbiAddress: sbiAddr, SbiPort: sbiPort}
	exists, err := s.DbIf.Exist(nfKeyPrefix + nfName)
	if err != nil {
		return err
	}
	if exists {
		previous, err := s.loadNF(nfName)
		if err != nil {
			return err
		}
		nf.Services = previous.Services
	}
	for _, service := range services {
		if !slices.Contains(nf.Services, service) {
			nf.Services = append(nf.Services, service)
		}
	}
	return s.saveNF(nf)
}

func (s *serviceContext) deregisterServices(nfName string, services []string) error {
	s.NsfLogger.SvcLog.Tracef("De-registering %s services", nfName)
	s.mu.Lock()
	defer s.mu.Unlock()

	exists, err := s.DbIf.Exist(nfKeyPrefix + nfName)
	if err != nil || !exists {
		return err
	}
	nf, err := s.loadNF(nfName)
	if err != nil {
		return err
	}
	nf.Services = slices.DeleteFunc(nf.Services, func(service string) bool {
		return slices.Contains(services, service)
	})
	if len(nf.Services) == 0 {
		return s.DbIf.Remove(nfKeyPrefix + nfName)
	}
	return s.saveNF(nf)
}

func (s *serviceContext) discoverService(service string) (string, string, int, error) {
	s.NsfLogger.SvcLog.Tracef("Discovering service %s", service)
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.DbIf.LoadAll()
	if err != nil {
		return "", "", 0, err
	}
	// Stable ordering also permits multiple NFs to provide the same service.
	keys := make([]string, 0, len(data))
	for key := range data {
		if strings.HasPrefix(key, nfKeyPrefix) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		var nf registeredNF
		if err := json.Unmarshal([]byte(data[key]), &nf); err != nil {
			return "", "", 0, fmt.Errorf("decode NF %s: %w", key, err)
		}
		if slices.Contains(nf.Services, service) {
			return nf.NfName, nf.SbiAddress, nf.SbiPort, nil
		}
	}
	return "", "", 0, fmt.Errorf("service %s not found", service)
}

func (s *serviceContext) loadNF(nfName string) (registeredNF, error) {
	value, err := s.DbIf.Load(nfKeyPrefix + nfName)
	if err != nil {
		return registeredNF{}, err
	}
	var nf registeredNF
	if err := json.Unmarshal([]byte(value), &nf); err != nil {
		return registeredNF{}, fmt.Errorf("decode NF %s: %w", nfName, err)
	}
	return nf, nil
}

func (s *serviceContext) saveNF(nf registeredNF) error {
	value, err := json.Marshal(nf)
	if err != nil {
		return err
	}
	return s.DbIf.Save(nfKeyPrefix+nf.NfName, string(value))
}
