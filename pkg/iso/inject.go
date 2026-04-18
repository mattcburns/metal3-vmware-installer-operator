/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iso

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InjectKsConfig injects a VMware kickstart configuration file into an ISO image.
// Uses xorriso to extract the existing EFI boot.cfg, patch the kernelopt= line to
// add ks=cdrom:/KS.CFG, and produce a new ISO containing both ks.cfg and the patched
// boot.cfg while preserving all El Torito / EFI boot records.
func InjectKsConfig(isoBlob []byte, ksConfig string) ([]byte, error) {
	if len(isoBlob) == 0 {
		return nil, fmt.Errorf("iso blob is empty")
	}

	if ksConfig == "" {
		return nil, fmt.Errorf("ksConfig is empty")
	}

	return injectKsConfigXorriso(isoBlob, ksConfig)
}

// injectKsConfigXorriso uses xorriso to:
//  1. Extract /EFI/BOOT/BOOT.CFG from the input ISO
//  2. Patch the kernelopt= line to add ks=cdrom:/KS.CFG
//  3. Produce a new ISO containing ks.cfg and the patched boot.cfg while
//     preserving all El Torito / EFI boot records from the original.
//
// See: https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/create-an-installer-iso-image-with-a-custom-installation-or-upgrade-script.html
func injectKsConfigXorriso(isoBlob []byte, ksConfig string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "vmware-iso-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.iso")
	if err := os.WriteFile(inPath, isoBlob, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write ISO to temp file: %w", err)
	}

	// List all files in the ISO so we can locate BOOT.CFG regardless of case or
	// directory depth. ESXi ISO layouts vary across versions.
	//nolint:gosec // paths constructed from os.MkdirTemp, not user input
	cmdFind := exec.Command("xorriso",
		"-indev", inPath,
		"-find", "/", "-type", "f",
		"--",
	)
	findOut, _ := cmdFind.CombinedOutput()

	// Find a path that contains "/efi/" and ends with "boot.cfg" (both case-insensitive).
	// xorriso prints absolute ISO paths wrapped in single quotes, one per line.
	efiBcISOPath := ""
	for _, line := range strings.Split(string(findOut), "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "'"))
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "/EFI/") && strings.HasSuffix(upper, "BOOT.CFG") {
			efiBcISOPath = line
			break
		}
	}
	if efiBcISOPath == "" {
		return nil, fmt.Errorf("EFI BOOT.CFG not found in ISO; full xorriso file list:\n%s", string(findOut))
	}
	fmt.Printf("Found EFI boot.cfg at ISO path: %s\n", efiBcISOPath)

	// Extract the EFI boot.cfg from the ISO.
	// -osirrox on enables output to the local filesystem.
	// -cpx copies a single file from the ISO to a local path.
	extractedBootCfg := filepath.Join(tmpDir, "boot.cfg.orig")
	//nolint:gosec // paths constructed from os.MkdirTemp, not user input
	cmdExtract := exec.Command("xorriso",
		"-indev", inPath,
		"-osirrox", "on",
		"-cpx", efiBcISOPath, extractedBootCfg,
		"--",
	)
	if out, err := cmdExtract.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to extract EFI boot.cfg: %w\n%s", err, string(out))
	}

	bcContent, err := os.ReadFile(extractedBootCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted EFI boot.cfg: %w", err)
	}

	patched := appendKsToKernelopt(string(bcContent))
	if patched == string(bcContent) {
		return nil, fmt.Errorf("failed to patch EFI boot.cfg: kernelopt= line not found or ks= already present")
	}

	ksLocalPath := filepath.Join(tmpDir, "ks.cfg")
	if err := os.WriteFile(ksLocalPath, []byte(ksConfig), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write ks.cfg temp file: %w", err)
	}

	bootCfgLocalPath := filepath.Join(tmpDir, "boot.cfg")
	if err := os.WriteFile(bootCfgLocalPath, []byte(patched), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write patched boot.cfg temp file: %w", err)
	}

	// Produce the output ISO with ks.cfg and the patched boot.cfg injected.
	// -boot_image any replay preserves El Torito / EFI boot records from -indev.
	outPath := filepath.Join(tmpDir, "output.iso")
	//nolint:gosec // paths constructed from os.MkdirTemp, not user input
	cmdInject := exec.Command("xorriso",
		"-indev", inPath,
		"-outdev", outPath,
		"-map", ksLocalPath, "/KS.CFG",
		"-map", bootCfgLocalPath, efiBcISOPath,
		"-boot_image", "any", "replay",
	)
	if out, err := cmdInject.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("xorriso inject failed: %w\n%s", err, string(out))
	}

	modifiedISO, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read modified ISO: %w", err)
	}

	fmt.Printf("Successfully injected ks.cfg into ISO using xorriso (size: %d -> %d bytes)\n",
		len(isoBlob), len(modifiedISO))

	return modifiedISO, nil
}

// appendKsToKernelopt appends ks=cdrom:/KS.CFG to the kernelopt= line in a boot.cfg
// file. If a ks= option is already present, or no kernelopt= line is found, the
// original content is returned unchanged. ESXi installer requires this option to
// locate the kickstart file on the CD-ROM without interactive input.
func appendKsToKernelopt(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "kernelopt=") {
			if strings.Contains(line, "ks=") {
				// Already references a kickstart location; leave unchanged.
				return content
			}
			lines[i] = strings.TrimRight(line, " \t") + " ks=cdrom:/KS.CFG"
			return strings.Join(lines, "\n")
		}
	}
	return content
}
