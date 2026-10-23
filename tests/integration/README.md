# Integration tests

Automated end-to-end tests of the addon Docker image, independent of a real HomeAssistant.
Everything runs with `docker compose` on a dedicated bridge network (`172.28.0.0/24`):

| service | purpose |
|---|---|
| `supervisor` | `supervisor_mock.py`: tiny fake of the HA Supervisor API used by `bashio` in the init scripts. It makes the addon start without any "Something went wrong contacting the API" error/timeout, and exercises the real `nginx-init`/`dnsmasq-init` code paths. |
| `addon` | the real addon image (real dnsmasq + backend + nginx), configured by `options.json`; `/data` is a volume that starts empty (leases are **not** faked). |
| `client-*` | one container per DHCP client (busybox `udhcpc`, fixed MAC/hostname) which obtains a lease from the real dnsmasq; the result is saved in a shared volume. One client uses a blocklisted MAC and must get no lease. |
| `tests` | pytest suite (`tests/`): DHCP leases/options, lease file, tracker DB, web UI + WebSocket, DNS (custom hosts, DHCP names, CNAME aliases). |

## Running

```bash
make FAST=1 build-docker-image    # builds the image <image>:localtest
make test-integration             # exit code is the one of the pytest run
make test-integration-clean       # remove leftover containers/volumes
```

If the tests fail, `make test-integration` prints the Compose container status and
timestamped service logs before cleaning up, in addition to pytest's failure output.

Requirements: docker with the compose plugin; the host must allow DHCP broadcasts and
UDP/53 traffic between containers on a bridge network (true on standard GitHub runners).

## Fixture

`fixtures/dnsmasq.leases` is the lease file written by the real dnsmasq for the clients above. Re-generate it with:

```bash
make test-integration-record-fixtures
```

It is used by a fast Go unit test (`backend/pkg/uibackend/leasefixture_test.go`), which is skipped while the file is missing.

## Keeping data in sync

MACs/hostnames/IPs appear in `options.json`, `docker-compose.yml` and `tests/conftest.py`.
