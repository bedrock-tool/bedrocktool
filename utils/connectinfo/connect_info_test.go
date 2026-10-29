package connectinfo

import "testing"

func TestParseConnectInfo(t *testing.T) {
	type test struct {
		value    string
		expected ParsedConnectInfo
	}
	var tests = []test{
		{value: "minecraft.net", expected: ParsedConnectInfo{Kind: KindServer, Address: "https://minecraft.net:443"}},
		{value: "minecraft.net:19133", expected: ParsedConnectInfo{Kind: KindServer, Address: "https://minecraft.net:19133"}},
		{value: "http://minecraft.net:19132", expected: ParsedConnectInfo{Kind: KindServer, Address: "http://minecraft.net:19132"}},
		{value: "https://minecraft.net", expected: ParsedConnectInfo{Kind: KindServer, Address: "https://minecraft.net:443"}},
		{value: "experience:Test Experience", expected: ParsedConnectInfo{Kind: KindExperience, Experience: "Test Experience"}},
		{value: "realm:test-realm", expected: ParsedConnectInfo{Kind: KindRealm, RealmName: "test-realm"}},
		{value: "realm:1234567", expected: ParsedConnectInfo{Kind: KindRealm, RealmName: "1234567"}},
		{value: "xbox:00000000-0000-0000-0000-000000000001", expected: ParsedConnectInfo{Kind: KindXbox, Xbox: "00000000-0000-0000-0000-000000000001"}},
		{value: "test-capture.pcap2", expected: ParsedConnectInfo{Kind: KindReplay, Replay: "test-capture.pcap2"}},
	}

	for _, tt := range tests {
		r, err := (&ConnectInfo{Value: tt.value}).Parsed()
		if err != nil {
			t.Fatal(err)
		}
		if *r != tt.expected {
			t.Fatalf("%s expected: %v\ngot: %v\n", tt.value, tt.expected, r)
		}
	}
}
