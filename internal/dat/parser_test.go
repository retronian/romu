package dat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDAT(t *testing.T) {
	xml := `<?xml version="1.0"?>
<datafile>
	<header>
		<name>Nintendo - Nintendo Entertainment System (Headered)</name>
		<description>Nintendo - NES</description>
	</header>
	<game name="Super Mario Bros. (World)">
		<rom name="Super Mario Bros. (World).nes" size="40976" crc="3337EC46" md5="811B027EAF99C2DEF7B933C5208636DE" sha1="FACEE9C577A5262DBE33AC4930BB0B58C8C037F7"/>
	</game>
	<game name="The Legend of Zelda (USA)">
		<rom name="The Legend of Zelda (USA).nes" size="131088" crc="A12D74C1" md5="4E1B0D2C4D1E2A4C5B6D7E8F9A0B1C2D" sha1="1234567890ABCDEF1234567890ABCDEF12345678"/>
	</game>
</datafile>`

	tmp := t.TempDir()
	datPath := filepath.Join(tmp, "test.dat")
	os.WriteFile(datPath, []byte(xml), 0644)

	roms, header, err := ParseDAT(datPath, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if header != "Nintendo - Nintendo Entertainment System (Headered)" {
		t.Errorf("unexpected header: %s", header)
	}

	if len(roms) != 2 {
		t.Fatalf("expected 2 roms, got %d", len(roms))
	}

	if roms[0].Platform != "FC" {
		t.Errorf("expected FC platform, got %s", roms[0].Platform)
	}
	if roms[0].GameTitle != "Super Mario Bros. (World)" {
		t.Errorf("unexpected title: %s", roms[0].GameTitle)
	}
	if roms[0].CRC32 != "3337EC46" {
		t.Errorf("unexpected crc: %s", roms[0].CRC32)
	}
}

func TestDetectPlatformFromHeader(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Nintendo - Game Boy Advance", "GBA"},
		{"Sega - Mega Drive - Genesis", "MD"},
		{"Nintendo - Super Nintendo Entertainment System", "SFC"},
		{"Unknown System", ""},
	}
	for _, tt := range tests {
		got := detectPlatformFromHeader(tt.name)
		if got != tt.want {
			t.Errorf("detectPlatformFromHeader(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
