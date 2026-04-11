# Test Fixtures

This directory contains test fixtures for the vmware-operator.

## ISO Fixtures

- `esxi_mock.iso` - Minimal mock ESXi ISO image for unit testing
  - Size: ~1KB (mock/placeholder)
  - Used for: ISO injection testing, ORAS client testing
  - Valid for ESXi 8.x and 9.x testing (same ks.cfg format)

These are minimal mock ISOs used for unit testing only. They are NOT valid bootable ISOs.
For integration testing against real Metal3/Ironic, use actual ESXi ISO images from VMware.
