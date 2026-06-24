package dialog

import "testing"

func TestExtractLongestCommonName_UserExample(t *testing.T) {
	names := []string{
		"[aaa] some common name - 01 - asdf asdf",
		"[bbb] some common name - 02 - asdf asdf",
		"[acc] some common name - 03 - asdf asdf",
	}
	got := ExtractLongestCommonName(names)
	if got != "some common name" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, "some common name")
	}
}

func TestExtractLongestCommonName_PrefixOnly(t *testing.T) {
	names := []string{
		"alpha-one.txt",
		"alpha-two.txt",
		"alpha-three.txt",
	}
	got := ExtractLongestCommonName(names)
	if got != "alpha" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, "alpha")
	}
}

func TestExtractLongestCommonName_NoMatch(t *testing.T) {
	names := []string{
		"apple.txt",
		"banana.txt",
		"cherry.txt",
	}
	got := ExtractLongestCommonName(names)
	if got != "" {
		t.Fatalf("ExtractLongestCommonName() = %q, want empty", got)
	}
}

func TestExtractLongestCommonName_TooFewNames(t *testing.T) {
	if got := ExtractLongestCommonName(nil); got != "" {
		t.Fatalf("nil names = %q, want empty", got)
	}
	if got := ExtractLongestCommonName([]string{"only.txt"}); got != "" {
		t.Fatalf("single name = %q, want empty", got)
	}
}

func TestExtractLongestCommonName_TwoNamesMiddle(t *testing.T) {
	names := []string{
		"prefix-shared-suffix",
		"other-shared-tail",
	}
	got := ExtractLongestCommonName(names)
	if got != "shared" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, "shared")
	}
}

func TestExtractLongestCommonName_RejectsSingleRune(t *testing.T) {
	names := []string{
		"x-alpha",
		"y-alpha",
	}
	got := ExtractLongestCommonName(names)
	if got != "alpha" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, "alpha")
	}
}

func TestExtractLongestCommonName_PreservesLeadingDigit(t *testing.T) {
	names := []string{
		"3D scene 01",
		"3D scene 02",
	}
	got := ExtractLongestCommonName(names)
	if got != "3D scene" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, "3D scene")
	}
}

func TestExtractLongestCommonName_PreservesLeadingDot(t *testing.T) {
	names := []string{
		".hidden-one",
		".hidden-two",
	}
	got := ExtractLongestCommonName(names)
	if got != ".hidden" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, ".hidden")
	}
}

func TestExtractLongestCommonName_PrefixFromNameStart(t *testing.T) {
	names := []string{
		"aproject 01",
		"aproject 02",
	}
	got := ExtractLongestCommonName(names)
	if got != "aproject" {
		t.Fatalf("ExtractLongestCommonName() = %q, want %q", got, "aproject")
	}
}
