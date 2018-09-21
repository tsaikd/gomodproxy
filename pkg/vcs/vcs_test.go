package vcs

import "testing"

func TestVersion(t *testing.T) {
	if !Version("v1.0.0").IsSemVer() {
		t.Fatal()
	}
	if Version("1.0.0").IsSemVer() {
		t.Fatal()
	}
	if Version("v0.0.0-20180910181607-0e37d006457b").IsSemVer() {
		t.Fatal()
	}
	if Version("v0.0.0-20180910181607-0e37d006457b").Hash() != "0e37d006457b" {
		t.Fatal()
	}
}
