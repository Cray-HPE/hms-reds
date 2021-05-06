# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.18.5] - 2021-05-06

### Changed

- Added istio pod anti-affinity to helm chart.

## [1.18.4] - 2021-04-15

### Changed

- Updated Dockerfile to pull base images from Artifactory instead of DTR.

## [1.18.3] - 2021-04-05

### Changed

- CASMHMS-4665 - Now properly handles when ETCD is not available when REDS starts up.

## [1.18.2] - 2021-03-31

### Changed

- CASMHMS-4605 - Update the loftsman/docker-kubectl image to use a production version.

## [1.18.1] - 2021-02-05

### Changed

- Added UserAgent headers to outbound HTTP requests.

## [1.18.0] - 2021-02-04

### Changed

- Updated Copyright/License in source files
- Re-vendored packages

## [1.17.2] - 2021-01-26

### Changed

- Changed the rsyslog DNS name to the new one.

## [1.17.1] - 2021-01-22

### Changed

- CASMHMS-4367 - REDS no longer marks redfishEndpoints as disabled in HSM when they are no longer on the network, as this is not necessary.

## [1.17.0] - 2021-01-14

### Changed

- Updated license file.


## [1.16.2] - 2020-11-13

- CASMHMS-4212 - Added final CA bundle configmap handling to Helm chart.

## [1.16.1] - 2020-11-02

### Security

- CASMHMS-4105 - Updated base Golang Alpine image to resolve libcrypto vulnerability.
- Removed old DNS/DHCP library code.

## [1.16.0] - 2020-10-21

- Removed the now-obsolete cray-init job.

## [1.15.0] - 2020-10-14

### Added

- Added TLS/CA support for Columbia Redfish operations.

## [1.14.2] - 2020-10-02

### Changed

- Upgraded the required cray-charts version to 2.0.1 to pick up upgrades to etcd

## [1.14.1] - 2020-10-01

### Security

- CASMHMS-4065 - Update base image to alpine-3.12.

## [1.14.0] - 2020-09-15

### Changed

- CASMCLOUD-1023 - changes for new base Cray charts.

## [1.13.1] - 2020-07-29

### Changed

- REDS now correctly sends HSM an xname in the FQDN field, rather than an IP.

## [1.13.0] - 2020-07-08

### Added

- Added the ability to recover from SNMP failures communicating with a switch
- Added periodic refreshes of SNMP switch data

## [1.12.2] - 2020-07-01

### Added

- CASMHMS-3630 - Updated REDS CT smoke test with new API test cases.

## [1.12.1] - 2020-06-30

### Changed

- Added extra information to log messages so users are better able to determine what infomration REDS is missing about particular nodes.

## [1.12.0] - 2020-06-26

### Changed

- Bumped dependant cray-service chart version
- Bumped minor version to account for diversion between master and 1.3

## [1.11.8] - 2020-06-11

### Removed

- As much ansible as possible from REDS (only image upload remains)
- All loader job logic (no more reds mapping or MaaS bridge
- The reliance on a wait-for-etcd pod
- The hardcoded credentials hack we had to use to set credentials on Gigabyte nodes

## [1.11.7] - 2020-06-08

### Fixed

- CASMHMS-3545 - Removed dependency on SMD loader job that has been removed.

## [1.11.6] - 2020-06-03

### Added

- CASMHMS-3262 - Enabled online upgrade/downgrade

## [1.11.5] - 2020-04-30

### Changed

- CASMHMS-2964 - Build images based off of trusted baseOS.

## [1.11.4] - 2020-04-24

- Fixed a change to the sls loader check.

## [1.11.3] - 2020-04-21

### Fixed

- CASMHMS-3334 - Don't panic in the case where we cannot do the xname port map, probably using SLS

## [1.11.2] - 2020-04-15

### Changed

- CASMHMS-3300 - Update CT smoke test for REDS running in SLS mode.

## [1.11.1] - 2020-04-08

### Changed

- CASMHMS-3283 - Changed how we handle credentials for Columbia switches to not overwrite values in Vault with Vault URLs.

## [1.11.0] - 2020-04-04

### Changed

- CASMHMS-3242 - Added support across the board for switch passwords in Vault. Also made NCNs work with SLS.

## [1.10.1] - 2020-03-30

### Changed

- CASMHMS-3196 - Fix a crash related to missing properties when pulling data from SLS

## [1.10.0] - 2020-03-27

### Added

- CASMHMS-3159 - Have REDS regularly poll SLS for Columbia switches.  This functionality should complete transitioning REDS to be able to pick up system changes via SLS as they happen and without restart.

## [1.9.0] - 2020-03-23

- CASMHMS-3161 - make REDS wait for SLS loader before coming up.

## [1.8.8] - 2020-03-09

- CASMHMS-3080 - do not panic on init is sls is not present, but add to readiness probe.

## [1.8.7] - 2020-03-06

- Added ability for SLS to be enabled if set in the global ConfigMap.

## [1.8.6] - 2020-03-04

- Now uses the lates hms-bmc-networkprotocol package to fix JSON rendering issues.

## [1.8.5] - 2020-03-02

### Changed

- CASMHMS-3012 - use the service name instead of a static IP address for remote logging.

## [1.8.4] - 2020-03-02

### Changed

- CASMPET-1988 - update cray-service dependency.

## [1.8.3] - 2020-02-27

### Changed

- Added picking up environment variables from a configmap for the cray-reds-init job

## [1.8.2] - 2020-02-17

### Fixed

- CASMHMS-2947 - incorrect name and port for system logging service.

## [1.8.1] - 2020-02-13

### Changed

- REDS now uses PATCHs instead of PUTs to modify existing redfish endpoints in HSM.
- REDS now triggers HSM rediscovery with the PATCH calls instead of POSTs to /Inventory/Discover

## [1.8.0] - 2020-01-17

### Added

- Added liveness and readiness probes

## [1.7.7] - 2020-01-10

### Fixed

- Removed the wait-for-gateway container from cray-reds-init since it is no
  longer needed and doesn't work since BSS has been removed from the whitelist.

## [1.7.6] - 2020-01-07

### Added

- New init container that waits for cray-smd-loader job to complete before allowing the REDS service to startup. This
  is a hacky workaround put in place to ensure that the NCNs get added correctly to HSM, however this should be fixed
  in code to cause this add attempt to retry rather than making REDS wait for SMD.

## [1.7.5] - 2019-12-18

### Fixed

- Change reds-init from using an IP address over to the hostname of the istio gateway

## [1.7.3] - 2019-12-03

### Fixed

- SNMP Passwords no longer print in logs and are stored in the Vault.

## [1.7.2] - 2019-12-02

### Changed

- Minor logging fixes

## [1.7.1] - 2019-11-25

### Fixed

- There was an issue where multiple switches with overlappoing layer-2 domains would report the same MAC address multiple times, leading to REDS being unable to determine which port it was actually attached to.  Better filtering of incoming MAC addresses resolved this issue.

## [1.7.4] - 2019-12-12

### Changed

- Updated hms-common lib.

## [1.7.0] - 2019-11-17

### Added

- Vault loader to transfer default credentials from Ansible to Kubernetes secret and finally to Hasicorp vault.

## [1.6.12] - 2019-11-26

### Changed

- Improved retry logic in loader to essentially retry forever.

## [1.6.11] - 2019-11-21

### Fixed

- Rediscovering nodes after they're administratively removed from HSM no longer requires restarting REDS.

## [1.6.10] - 2019-11-13

### Fixed

- Now the Switches with bad software versions will cause REDS to throw a panic

## [1.6.9] - 2019-11-13

### Fixed

- Now returns 400 errors which were previously incorrectly 405.  Implemented 405 errors for bad methods on API endpoints.

## [1.6.8] - 2019-11-12

### Fixed

- Now the AccountService/Accounts Redfish endpoint is conformed to the HW vendor of the compute blade.

## [1.6.7] - 2019-11-08

### Fixed

- The REDS-MaaS bridge now sets RediscoverOnUpdate = true so that items get discovered after being added to HSM.

## [1.6.6] - 2019-11-01

### Added

- Ability to read from SLS instead of the mapping file.

## [1.6.5] - 2019-10-31

### Changed

- Remove references to kvstore
- Update HSM/BSS URLs to not use the gateway

## [1.6.4] - 2019-10-25

### Changed

- Changed defaults related to reaching SLS (bugfix)

## [1.6.3] - 2019-10-15

### Added

- Support for Columbia switch discovery

## [1.6.2] - 2019-10-07

### Changed

- Remove sensitive information from logging statements.

## [1.6.1] - 2019-10-02

### Changed

- Fix for handling of SNMP tables under Dell OS10.  These show all MAC addresses as other, rather than "learned" as Dell OS9 did.  This change accepts all kinds of mac addresses, but instead verifies they're associated with a valid port number.

## [1.6.0] - 2019-09-26

### Added

- Switch the storage for MAC and Global credentials to use Vault instead
of etcd or kvstore. The HSMNotification now posts without the username or password to
signal HSM to get passwords from Vault instead of the payload.

## [1.5.1] - 2019-08-19

### Added

- A quick hack to try known credentials and use any valid known credentials to set the per-node credentials.  This is not very secure and should
  not persist beyond v1.0!

## [1.5.0] - 2019-08-12

### Added

- Added new loader utility which is used to load REDS mapping file.

## [1.4.2] - 2019-08-08

### Added

- Ability to set DNS and DHCP records using SMNetManager service.  This feature is active only if the cmdline option -smnet <url> is set.

## [1.4.1] - 2019-08-05

### Added

- Read MAAS discovery info from config mapping "reds-maas-bridge-config".  To turn on reading this, set REDS_MAAS_BRIDGE_ENABLE non-empty.
## [1.4.0] - 2019-07-23

### Added

- Add the capability to use etcd as a backing store.  Switching between this and KVStore is accomplished using the DATASTORE_BACKEND environemnt variable in values.yaml.  This is now set to default to etcd

## [1.3.0] - 2019-07-18

This version is a highly artificial bump due to a big chunk of code getting reverted and version mismatch errors.

### Fixed

- Fixed incorrect HSM URL.

## [1.1.1] - 2019-05-29

### Changed

- Converted ReST calls to use the RESTY API

## [1.1.0] - 2019-05-14

### Changed

- Targeted v1.1.0 of `hms-common`.

## [1.0.0] - 2019-05-13

### Added

- This is the initial release. It contains everything that was in `hms-services` at the time with the major exception of being `go mod` based now.

### Changed

### Deprecated

### Removed

### Fixed

### Security
