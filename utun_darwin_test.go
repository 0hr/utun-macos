package tun_macos

import "testing"

func Test_Utun_Name(t *testing.T) {
	_, err := Open("tun")

	if err == nil {
		t.Error("it must have an error -> interface name must be utun[0-9]")
	}

	t.Logf(err.Error())
}

func Test_Utun_Unit(t *testing.T) {
	_, err := Open("utun")

	if err == nil {
		t.Error("it should have error for utun unit number -> interface name must have unit number")
	}
	t.Logf(err.Error())
}

func Test_Open(t *testing.T) {
	_, err := Open("utun10")

	if err != nil {
		t.Errorf("It should have been started successfully but there is an error: %s", err.Error())
	}
}
