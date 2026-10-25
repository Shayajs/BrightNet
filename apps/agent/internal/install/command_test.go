package install

import "testing"

func TestStartupValueQuotesTheExecutable(t *testing.T) {
	got := StartupValue(`C:\Program Files\BrightNet\BrightNetNodeParameter.exe`)
	want := `"C:\Program Files\BrightNet\BrightNetNodeParameter.exe" --background`
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestSameExecutableIgnoresCase(t *testing.T) {
	exe := `C:\Users\espin\AppData\Local\BrightNet\BrightNetNodeParameter.exe`
	value := `"c:\users\espin\appdata\local\brightnet\BrightNetNodeParameter.exe" --background`
	if !SameExecutable(value, exe) {
		t.Fatal("same file")
	}
	if SameExecutable(`"C:\other.exe" --background`, exe) {
		t.Fatal("other file")
	}
}
