package auth

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	a, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || a == "correct horse battery" {
		t.Fatal("password must be stored as a salted hash")
	}
	b, _ := HashPassword("correct horse battery")
	if a == b {
		t.Fatal("hashes need unique salts")
	}
	if !CheckPassword(a, "correct horse battery") || CheckPassword(a, "incorrect password") || CheckPassword("malformed", "x") {
		t.Fatal("password verification broken")
	}
	for _, p := range []string{"short", string(make([]byte, 1025))} {
		if _, err := HashPassword(p); err == nil {
			t.Fatal("invalid password accepted")
		}
	}
}
func TestPasswordWorkIsBoundedWithoutWaiting(t *testing.T) {
	start := make(chan struct{})
	results := make(chan error, 32)
	for range 32 {
		go func() { <-start; _, e := HashPassword("correct horse battery"); results <- e }()
	}
	close(start)
	busy := 0
	for range 32 {
		if e := <-results; e != nil {
			if e != ErrBusy {
				t.Fatal(e)
			}
			busy++
		}
	}
	if busy == 0 {
		t.Fatal("unbounded password work admitted all concurrent requests")
	}
}
