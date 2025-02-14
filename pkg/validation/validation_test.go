package validation

import (
	"testing"

	"github.com/intelchain-itc/itc-sdk/pkg/sharding"
)

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		str string
		exp bool
	}{
		{"itc1yvhj85pr9nat6g0cwtd9mqhaj3whpgwwyacn6l", true},
		{"itc17jknjqzzzqwxr6dq95syyahzqx9apjca4rhhff", true},
		{"itcfoofoo", false},
		{"0xbarbar", false},
		{"dsasdadsasaadsas", false},
		{"32312123213213212321", false},
	}

	for _, test := range tests {
		err := ValidateAddress(test.str)
		valid := false

		if err == nil {
			valid = true
		}

		if valid != test.exp {
			t.Errorf(`ValidateAddress("%s") returned %v, expected %v`, test.str, valid, test.exp)
		}
	}
}

func TestIsValidShard(t *testing.T) {
	if err := ValidateNodeConnection("http://localhost:9500"); err != nil {
		t.Skip()
	}
	s, _ := sharding.Structure("http://localhost:9500")

	tests := []struct {
		shardID uint32
		exp     bool
	}{
		{0, true},
		{1, true},
		{98, false},
		{99, false},
	}

	for _, test := range tests {
		valid := ValidShardID(test.shardID, uint32(len(s)))

		if valid != test.exp {
			t.Errorf("ValidShardID(%d) returned %v, expected %v", test.shardID, valid, test.exp)
		}
	}
}
