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
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildTestISO creates a minimal ISO with /EFI/BOOT/BOOT.CFG using xorriso.
// The BOOT.CFG contains a kernelopt= line so injection can be tested end-to-end.
// The test is skipped if xorriso is not available on the host.
func buildTestISO(t *testing.T) []byte {
	t.Helper()

	if _, err := exec.LookPath("xorriso"); err != nil {
		t.Skip("xorriso not available on this host")
	}

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "iso-root")

	efiDir := filepath.Join(rootDir, "EFI", "BOOT")
	if err := os.MkdirAll(efiDir, 0o755); err != nil {
		t.Fatalf("failed to create EFI dir: %v", err)
	}

	bootCfg := "bootstate=0\nkernelopt=runweasel\ntitle=Test ESXi\n"
	if err := os.WriteFile(filepath.Join(efiDir, "BOOT.CFG"), []byte(bootCfg), 0o644); err != nil {
		t.Fatalf("failed to write BOOT.CFG: %v", err)
	}

	isoPath := filepath.Join(tmpDir, "test.iso")
	//nolint:gosec // tmpDir from t.TempDir(), not user input
	cmd := exec.Command("xorriso", "-as", "mkisofs", "-o", isoPath, "-J", "-r", rootDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create test ISO: %v\n%s", err, string(out))
	}

	data, err := os.ReadFile(isoPath)
	if err != nil {
		t.Fatalf("failed to read test ISO: %v", err)
	}
	return data
}

func TestInjectKsConfig(t *testing.T) {
	isoBlob := buildTestISO(t)
	ksConfig := "accepteula\nrootpw password\ninstall --firstdisk --overwritevmfs\nreboot\n"

	result, err := InjectKsConfig(isoBlob, ksConfig)
	if err != nil {
		t.Fatalf("InjectKsConfig returned error: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("InjectKsConfig returned empty result")
	}

	// Verify ks.cfg content is present in the output ISO by extracting it with xorriso
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "result.iso")
	if err := os.WriteFile(isoPath, result, 0o644); err != nil {
		t.Fatalf("failed to write result ISO: %v", err)
	}

	extractedKs := filepath.Join(tmpDir, "ks.cfg")
	//nolint:gosec
	cmd := exec.Command("xorriso", "-indev", isoPath, "-osirrox", "on", "-cpx", "/KS.CFG", extractedKs, "--")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to extract KS.CFG from result ISO: %v\n%s", err, string(out))
	}

	ksData, err := os.ReadFile(extractedKs)
	if err != nil {
		t.Fatalf("failed to read extracted KS.CFG: %v", err)
	}
	if !bytes.Equal(ksData, []byte(ksConfig)) {
		t.Errorf("KS.CFG content mismatch:\ngot:  %q\nwant: %q", string(ksData), ksConfig)
	}

	// Verify boot.cfg was patched with ks=cdrom:/KS.CFG
	extractedBootCfg := filepath.Join(tmpDir, "boot.cfg")
	//nolint:gosec
	cmd = exec.Command("xorriso", "-indev", isoPath, "-osirrox", "on", "-cpx", "/EFI/BOOT/BOOT.CFG", extractedBootCfg, "--")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to extract EFI/BOOT/BOOT.CFG from result ISO: %v\n%s", err, string(out))
	}

	bootCfgData, err := os.ReadFile(extractedBootCfg)
	if err != nil {
		t.Fatalf("failed to read extracted BOOT.CFG: %v", err)
	}
	if !bytes.Contains(bootCfgData, []byte("ks=cdrom:/KS.CFG")) {
		t.Errorf("BOOT.CFG was not patched with ks=cdrom:/KS.CFG:\n%s", string(bootCfgData))
	}
}

func TestInjectKsConfigEmptyISO(t *testing.T) {
	_, err := InjectKsConfig([]byte{}, "config")
	if err == nil {
		t.Error("InjectKsConfig should return error for empty ISO")
	}
}

func TestInjectKsConfigEmptyConfig(t *testing.T) {
	_, err := InjectKsConfig([]byte("iso-data"), "")
	if err == nil {
		t.Error("InjectKsConfig should return error for empty config")
	}
}

func TestAppendKsToKernelopt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		modified bool
	}{
		{
			name:     "adds ks= to plain kernelopt line",
			input:    "bootstate=0\nkernelopt=runweasel\ntitle=ESXi\n",
			want:     "bootstate=0\nkernelopt=runweasel ks=cdrom:/KS.CFG\ntitle=ESXi\n",
			modified: true,
		},
		{
			name:     "does not duplicate existing ks=",
			input:    "kernelopt=runweasel ks=cdrom:/KS.CFG\n",
			want:     "kernelopt=runweasel ks=cdrom:/KS.CFG\n",
			modified: false,
		},
		{
			name:     "handles kernelopt with trailing spaces",
			input:    "kernelopt=runweasel   \ntitle=ESXi",
			want:     "kernelopt=runweasel ks=cdrom:/KS.CFG\ntitle=ESXi",
			modified: true,
		},
		{
			name:     "no kernelopt line returns unchanged",
			input:    "bootstate=0\ntitle=ESXi\n",
			want:     "bootstate=0\ntitle=ESXi\n",
			modified: false,
		},
		{
			name:     "does not modify existing ks=usb: reference",
			input:    "kernelopt=ks=usb:/KS.CFG\n",
			want:     "kernelopt=ks=usb:/KS.CFG\n",
			modified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendKsToKernelopt(tt.input)
			if got != tt.want {
				t.Errorf("appendKsToKernelopt() = %q, want %q", got, tt.want)
			}
			if tt.modified && got == tt.input {
				t.Errorf("appendKsToKernelopt() should have modified input but did not")
			}
			if !tt.modified && got != tt.input {
				t.Errorf("appendKsToKernelopt() should not have modified input but did")
			}
		})
	}
}
