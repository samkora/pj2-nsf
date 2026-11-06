# Network Service Function

This is a network function used for network service register/de-register/discovery.

## Project 2 implementation

- `serviceContext` stores each NF as a JSON record under the key `nf:<nf_name>` in `DbIf`.
- Registration merges service names without duplicates. Deregistration removes only the requested services of the named NF; the record is deleted when its last service is removed.
- Discovery reads persisted records, so restarting NSF preserves registrations. If several NFs offer the same service, discovery selects the first NF in name order; removing that provider leaves the other providers discoverable.
- An `RWMutex` serializes changes within one NSF process and permits concurrent discovery. The course workload is small, so discovery scans the stored NF records rather than maintaining a second index that could become stale after a crash.
- `config.yaml` keeps the required `db_type: "bbolt"`. `config.mongodb.yaml` selects the MongoDB implementation without changing the default.

### MongoDB interface demo

Use an isolated local MongoDB server, then start NSF with the alternate configuration:

```bash
docker run -d --name nsf-project2-mongo --cpus=1 --memory=1g \
  -p 127.0.0.1:27018:27017 mongo:8.0
make
./build/nsf -c config.mongodb.yaml
```

The default URI is `mongodb://127.0.0.1:27018`, database `nsf_project2`, collection `services`.
For a different local test instance, use `NSF_MONGODB_URI` and `NSF_MONGODB_DATABASE` environment variables. Do not commit credentials.

In another terminal, register and inspect the actual database:

```bash
curl -i -X POST http://127.0.0.1:8000/sbi/nnsf-nfm \
  -H 'Content-Type: application/json' \
  -d '{"nf_name":"demo-nf","sbi_address":"127.0.0.2","sbi_port":8081,"services":["demo-service"]}'
docker exec nsf-project2-mongo mongosh --quiet nsf_project2 \
  --eval 'db.services.find().forEach(d => printjson({_id: d._id, data: JSON.parse(d.value)}))'
curl -i -X GET http://127.0.0.1:8000/sbi/nnsf-disc \
  -H 'Content-Type: application/json' -d '{"service":"demo-service"}'
curl -i -X DELETE http://127.0.0.1:8000/sbi/nnsf-nfm \
  -H 'Content-Type: application/json' -d '{"nf_name":"demo-nf","services":["demo-service"]}'
docker exec nsf-project2-mongo mongosh --quiet nsf_project2 \
  --eval 'db.services.countDocuments({_id:"nf:demo-nf"})'
```

The last count should be `0`. Record the config selection, NSF startup, registration, database query, discovery, deletion and final database count. Stop NSF with Ctrl+C and normally stop only the demo container when finished. Keep `config.yaml` on bbolt for grading.

### Additional verification

```bash
make lint
make
make test
go test -race ./internal/nsfContext ./internal/sbi
# Run only against a disposable database with this required name prefix:
NSF_TEST_MONGODB=1 NSF_MONGODB_DATABASE=nsf_project2_test_local go test -count=1 -v ./internal/db
```

MongoDB tests are skipped unless explicitly enabled. Tests cover restart recovery, partial deregistration, multiple providers, concurrent registration, HTTP status/payloads and failure to initialize the configured database. TA hidden tests are not included in this repository.

## SBI Api Endpoint Point

### API Table

| Service | Endpoint | Description | Model |
| - | - | - | - |
| Heartbeat | `/sbi/heartbeat` | health check | - |
| NF Management | `/sbi/nnsf-nfm` | register & de-register network service | [nnsf-nfm](#nnsf-nfm) |
| Service Discovery | `/sbi/nnsf-disc` | discover the service information | [nnsf-disc](#nnsf-disc) |
| Group Service Discovery | `/sbi/nnsf-disc/group` | discover multiple services information | [nnsf-disc](#nnsf-disc) |

### API Structure

```bash
/sbi
    └─/heartbeat(GET)
    └─/nnsf-nfm(POST\DELETE)
    └─/nnsf-disc(Get)
        └─/group(Get)
```

## Make Command

| Command | Description |
| - | - |
| `make` | default set to `make bin` |
| `make bin` | build the executable binary file and output under **build** folder |
| `make clean` | clean up the build folder |
| `make lint` | run golang-lint |
| `make test` | run go test through the whole repo |
| `make tidy` | run module tidy |
| `make run` | run the NSF binary file under **build** folder with [config.yaml](./config.yaml) |
| `make cleandb` | remove the bbolt db file |
| `make cleantestdb` | remove the test bbolt db files |

## Assignment implementation checklist

The original assignment's implementation points are completed:

1. `internal/nsfContext/serviceContext.go`: This file is the core context structure for manage the NF services. You have to implement it correctly which can pass the unit test in `serviceContext_test.go` and the integration test with the docker image provided by CNDI.
2. `internal/db/mongoDb.go`: Implements the complete `DbIf` interface using MongoDB and is selected by `db_type: "mongodb"`.
3. Use the `make` related command to make sure everything is correct.
4. Use the docker image provided by CNDI to validate if your NSF is implement correctly.

## Docker Image Validate

- Pull the image

    ```bash
    docker pull alonza0314/nsf-tester:latest
    ```

- Run the image for testing with local directory

    ```bash
    docker run --rm -v <absolute path to your local directory>:/work alonza0314/nsf-tester:latest
    ```

## SBI Model

### nnsf-nfm

- POST Request

    ```go
    type NnsfNfmPostRequest struct {
        NfName     string   `json:"nf_name"`
        SbiAddress string   `json:"sbi_address"`
        SbiPort    int      `json:"sbi_port"`
        Services   []string `json:"services"`
    }
    ```

- POST Response

    ```go
    type NnsfNfmPostResponse struct {
        ServiceToken string `json:"service_token"`
        ServiceUuid  string `json:"service_uuid"`
    }
    ```

- DELETE Request

    ```go
    type NnsfNfmDeleteRequest struct {
        NfName   string   `json:"nf_name"`
        Services []string `json:"services"`
    }
    ```

### nnsf-disc

- GET Request

    ```go
    type NnsfDiscGetRequest struct {
        Service string `json:"service"`
    }

    type NnsfDiscGroupGetRequest struct {
        Services []string `json:"services"`
    }
    ```

- GET Response

    ```go
    type NnsfDiscGetResponse struct {
        Service    string `json:"service"`
        NfName     string `json:"nf_name"`
        SbiAddress string `json:"sbi_address"`
        SbiPort    int    `json:"sbi_port"`
    }

    type NnsfDiscGroupGetResponse struct {
        Services []NnsfDiscGetResponse `json:"services"`
    }
    ```
