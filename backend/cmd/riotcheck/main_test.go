package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitRiotID(t *testing.T) {
	cases := []struct {
		riotID       string
		wantGameName string
		wantTagLine  string
	}{
		{"Faker#KR1", "Faker", "KR1"},
		{"Test Summoner#EUW", "Test Summoner", "EUW"},
		{"Игрок#RU1", "Игрок", "RU1"},
		{"a#b#c", "a#b", "c"},
	}

	for _, tc := range cases {
		t.Run(tc.riotID, func(t *testing.T) {
			gameName, tagLine, err := splitRiotID(tc.riotID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantGameName, gameName)
			assert.Equal(t, tc.wantTagLine, tagLine)
		})
	}
}

func TestSplitRiotIDInvalid(t *testing.T) {
	for _, riotID := range []string{"", "Faker", "#KR1", "Faker#", "#"} {
		t.Run("id="+riotID, func(t *testing.T) {
			_, _, err := splitRiotID(riotID)
			assert.Error(t, err)
		})
	}
}
