package wifi

import (
	"errors"
	"testing"
)

type mockRunner struct {
	output []byte
	err    error
}

func (m mockRunner) Output(name string, args ...string) ([]byte, error) {
	return m.output, m.err
}

func TestParseLinuxSSID_Connected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("yes:MyHomeNetwork\nno:OtherNetwork\n")}

	ssid, err := currentSSIDLinux()
	if err != nil {
		t.Fatalf("currentSSIDLinux() error = %v", err)
	}
	if ssid != "MyHomeNetwork" {
		t.Fatalf("currentSSIDLinux() = %q, want %q", ssid, "MyHomeNetwork")
	}
}

func TestParseLinuxSSID_Disconnected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("no:SomeNetwork\n")}

	ssid, err := currentSSIDLinux()
	if err != nil {
		t.Fatalf("currentSSIDLinux() error = %v", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDLinux() = %q, want empty string", ssid)
	}
}

func TestParseLinuxSSID_MultipleActive(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("yes:FirstNetwork\nyes:SecondNetwork\n")}

	ssid, err := currentSSIDLinux()
	if err != nil {
		t.Fatalf("currentSSIDLinux() error = %v", err)
	}
	if ssid != "FirstNetwork" {
		t.Fatalf("currentSSIDLinux() = %q, want %q", ssid, "FirstNetwork")
	}
}

func TestParseLinuxSSID_CommandFails(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{err: errors.New("command failed")}

	ssid, err := currentSSIDLinux()
	if err != nil {
		t.Fatalf("currentSSIDLinux() error = %v, want nil", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDLinux() = %q, want empty string", ssid)
	}
}

func TestParseLinuxSSID_EmptyOutput(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("")}

	ssid, err := currentSSIDLinux()
	if err != nil {
		t.Fatalf("currentSSIDLinux() error = %v", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDLinux() = %q, want empty string", ssid)
	}
}

func TestParseDarwinSSID_Connected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("     agrCtlRSSI: -55\n     agrExtRSSI: 0\n    agrCtlNoise: -88\n    agrExtNoise: 0\n          state: running\n        op mode: station\n     lastTxRate: 867\n        maxRate: 867\nlastAssocStatus: 0\n    802.11 auth: open\n      link auth: wpa2-psk\n          BSSID: aa:bb:cc:dd:ee:ff\n           SSID: MyOfficeWifi\n            MCS: 9\n        channel: 149,80\n")}

	ssid, err := currentSSIDDarwin()
	if err != nil {
		t.Fatalf("currentSSIDDarwin() error = %v", err)
	}
	if ssid != "MyOfficeWifi" {
		t.Fatalf("currentSSIDDarwin() = %q, want %q", ssid, "MyOfficeWifi")
	}
}

func TestParseDarwinSSID_Disconnected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("     agrCtlRSSI: -55\n          state: init\n          BSSID: 00:00:00:00:00:00\n")}

	ssid, err := currentSSIDDarwin()
	if err != nil {
		t.Fatalf("currentSSIDDarwin() error = %v", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDDarwin() = %q, want empty string", ssid)
	}
}

func TestParseDarwinSSID_CommandFails(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{err: errors.New("command failed")}

	ssid, err := currentSSIDDarwin()
	if err != nil {
		t.Fatalf("currentSSIDDarwin() error = %v, want nil", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDDarwin() = %q, want empty string", ssid)
	}
}

// --- PowerShell-based WSL tests ---

func TestParsePowerShellSSID_Connected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("MyHomeNetwork\r\n")}

	ssid, err := currentSSIDPowerShell()
	if err != nil {
		t.Fatalf("currentSSIDPowerShell() error = %v", err)
	}
	if ssid != "MyHomeNetwork" {
		t.Fatalf("currentSSIDPowerShell() = %q, want %q", ssid, "MyHomeNetwork")
	}
}

func TestParsePowerShellSSID_Disconnected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("\r\n")}

	ssid, err := currentSSIDPowerShell()
	if err != nil {
		t.Fatalf("currentSSIDPowerShell() error = %v", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDPowerShell() = %q, want empty string", ssid)
	}
}

func TestParsePowerShellSSID_CommandFails(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{err: errors.New("command failed")}

	ssid, err := currentSSIDPowerShell()
	if err == nil {
		t.Fatal("currentSSIDPowerShell() expected error, got nil")
	}
	if ssid != "" {
		t.Fatalf("currentSSIDPowerShell() = %q, want empty string", ssid)
	}
}

// --- WSL integration tests (PowerShell → netsh fallback) ---

type sequenceRunner struct {
	calls   int
	outputs []mockRunner
}

func (s *sequenceRunner) Output(name string, args ...string) ([]byte, error) {
	i := s.calls
	s.calls++
	if i < len(s.outputs) {
		return s.outputs[i].output, s.outputs[i].err
	}
	return nil, errors.New("no more mock outputs")
}

func TestWSL_PowerShellSucceeds(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("MyHomeNetwork\r\n")}

	ssid, err := currentSSIDWSL()
	if err != nil {
		t.Fatalf("currentSSIDWSL() error = %v", err)
	}
	if ssid != "MyHomeNetwork" {
		t.Fatalf("currentSSIDWSL() = %q, want %q", ssid, "MyHomeNetwork")
	}
}

func TestWSL_PowerShellFails_FallsBackToNetsh(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = &sequenceRunner{outputs: []mockRunner{
		{err: errors.New("powershell not found")},
		{output: []byte("    SSID                   : NetshNetwork\r\n    BSSID                  : aa:bb:cc:dd:ee:ff\r\n")},
	}}

	ssid, err := currentSSIDWSL()
	if err != nil {
		t.Fatalf("currentSSIDWSL() error = %v", err)
	}
	if ssid != "NetshNetwork" {
		t.Fatalf("currentSSIDWSL() = %q, want %q", ssid, "NetshNetwork")
	}
}

func TestWSL_PowerShellEmpty_FallsBackToNetsh(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = &sequenceRunner{outputs: []mockRunner{
		{output: []byte("\r\n")},
		{output: []byte("    SSID                   : NetshNetwork\r\n    BSSID                  : aa:bb:cc:dd:ee:ff\r\n")},
	}}

	ssid, err := currentSSIDWSL()
	if err != nil {
		t.Fatalf("currentSSIDWSL() error = %v", err)
	}
	if ssid != "NetshNetwork" {
		t.Fatalf("currentSSIDWSL() = %q, want %q", ssid, "NetshNetwork")
	}
}

func TestWSL_BothFail(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = &sequenceRunner{outputs: []mockRunner{
		{err: errors.New("powershell not found")},
		{err: errors.New("netsh not found")},
	}}

	ssid, err := currentSSIDWSL()
	if err != nil {
		t.Fatalf("currentSSIDWSL() error = %v, want nil", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDWSL() = %q, want empty string", ssid)
	}
}

// --- Netsh-only tests ---

func TestParseNetshSSID_Connected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("    Name                   : Wi-Fi\r\n    Description            : Intel(R) Wi-Fi 6 AX201 160MHz\r\n    GUID                   : abcdef\r\n    Physical address       : aa:bb:cc:dd:ee:ff\r\n    State                  : connected\r\n    SSID                   : MyHomeNetwork\r\n    BSSID                  : aa:bb:cc:dd:ee:ff\r\n    Network type           : Infrastructure\r\n")}

	ssid, err := currentSSIDNetsh()
	if err != nil {
		t.Fatalf("currentSSIDNetsh() error = %v", err)
	}
	if ssid != "MyHomeNetwork" {
		t.Fatalf("currentSSIDNetsh() = %q, want %q", ssid, "MyHomeNetwork")
	}
}

func TestParseNetshSSID_Disconnected(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("    Name                   : Wi-Fi\r\n    Description            : Intel(R) Wi-Fi 6 AX201 160MHz\r\n    State                  : disconnected\r\n")}

	ssid, err := currentSSIDNetsh()
	if err != nil {
		t.Fatalf("currentSSIDNetsh() error = %v", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDNetsh() = %q, want empty string", ssid)
	}
}

func TestParseNetshSSID_CommandFails(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{err: errors.New("command failed")}

	ssid, err := currentSSIDNetsh()
	if err != nil {
		t.Fatalf("currentSSIDNetsh() error = %v, want nil", err)
	}
	if ssid != "" {
		t.Fatalf("currentSSIDNetsh() = %q, want empty string", ssid)
	}
}

func TestParseNetshSSID_SkipsBSSID(t *testing.T) {
	orig := runner
	t.Cleanup(func() { runner = orig })
	runner = mockRunner{output: []byte("    SSID                   : MyNetwork\r\n    BSSID                  : aa:bb:cc:dd:ee:ff\r\n")}

	ssid, err := currentSSIDNetsh()
	if err != nil {
		t.Fatalf("currentSSIDNetsh() error = %v", err)
	}
	if ssid != "MyNetwork" {
		t.Fatalf("currentSSIDNetsh() = %q, want %q", ssid, "MyNetwork")
	}
}
