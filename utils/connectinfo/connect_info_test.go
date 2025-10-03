package connectinfo

import "testing"

func TestParseConnectInfo(t *testing.T) {
	type test struct {
		value    string
		expected parsedConnectInfo
	}
	var tests = []test{
		{value: "minecraft.net", expected: parsedConnectInfo{serverAddress: "minecraft.net:19132"}},
		{value: "realm:test-realm", expected: parsedConnectInfo{realmName: "test-realm"}},
		{value: "gathering:test-gathering", expected: parsedConnectInfo{gatheringName: "test-gathering"}},
		{value: "test-capture.pcap2", expected: parsedConnectInfo{replayName: "test-capture.pcap2"}},
	}

	for _, tt := range tests {
		r, err := parseConnectInfo(tt.value)
		if err != nil {
			t.Fatal(err)
		}
		if *r != tt.expected {
			t.Fatalf("%s expected: %v\ngot: %v\n", tt.value, tt.expected, r)
		}
	}
}
