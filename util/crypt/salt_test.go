package crypt

import "testing"

var _Salt = &Salt{
	MagicPrefix: []byte("$foo$"),
	SaltLenMin:  1,
	SaltLenMax:  8,
}

func TestGenerateSalt(t *testing.T) {
	magicPrefixLen := len(_Salt.MagicPrefix)

	salt := _Salt.Generate(0)
	if len(salt) != magicPrefixLen+1 {
		t.Errorf("Expected len 1, got len %d", len(salt))
	}

	for i := _Salt.SaltLenMin; i <= _Salt.SaltLenMax; i++ {
		salt = _Salt.Generate(i)
		if len(salt) != magicPrefixLen+i {
			t.Errorf("Expected len %d, got len %d", i, len(salt))
		}
	}

	salt = _Salt.Generate(9)
	if len(salt) != magicPrefixLen+8 {
		t.Errorf("Expected len 8, got len %d", len(salt))
	}
}
