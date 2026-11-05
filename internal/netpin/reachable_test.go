package netpin

import "testing"

func TestReachable(t *testing.T) {
	defer Set(0, 0)
	Set(0, 0)
	if !Reachable(true) || !Reachable(false) {
		t.Fatal("без VPN проверять нечего")
	}
	Set(5, 0) // VPN включён, привязка только для IPv4
	if !Reachable(false) {
		t.Fatal("IPv4 привязан")
	}
	if Reachable(true) {
		t.Fatal("IPv6 без привязки уйдёт в туннель")
	}
	Set(5, 7)
	if !Reachable(true) {
		t.Fatal("IPv6 привязан")
	}
}
