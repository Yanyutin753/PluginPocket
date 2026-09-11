package demo

import (
	"context"
	"fmt"
)

// Expand adds volume to an existing seed once. The first username is the guard
// against repeating financial changes; interrupted runs remain visible for review.
func Expand(ctx context.Context, origin, username, password string) error {
	if err := localOrigin(origin); err != nil {
		return err
	}
	s := &seeder{ctx: ctx, origin: origin}
	admin := client()
	me := s.api(admin, "POST", "/auth/login", map[string]any{"username": username, "password": password}, 200)
	defer s.logout(admin)
	if s.err != nil {
		return s.err
	}
	// Register this account before changing any administrator data. A second run
	// fails here, leaving all existing money, tools and credentials untouched.
	first := client()
	s.api(first, "POST", "/auth/register", map[string]any{"username": "demo_user_01", "password": Password}, 201)
	defer s.logout(first)
	if s.err != nil {
		return s.err
	}
	s.api(admin, "POST", fmt.Sprintf("/admin/users/%d/balance", me.User.ID), map[string]any{"delta": 10000, "note": "DEMO: administrator working balance", "idempotency_key": "demo-admin"}, 200)
	var active string
	for i := 1; i <= 60; i++ {
		if s.err != nil {
			return s.err
		}
		token := s.api(admin, "POST", "/account/tokens", map[string]any{"name": fmt.Sprintf("DEMO workstation %02d", i)}, 201)
		if i == 1 {
			active = token.Token
		}
		if i%5 == 0 {
			s.api(admin, "DELETE", fmt.Sprintf("/account/tokens/%d", token.Item.ID), nil, 204)
		}
	}
	for i := 1; i <= 20; i++ {
		if s.err != nil {
			return s.err
		}
		s.call(active, "echo", map[string]any{"message": fmt.Sprintf("DEMO admin workflow %02d", i)}, false)
	}
	for i := 1; i <= 11; i++ {
		if s.err != nil {
			return s.err
		}
		team := s.api(admin, "POST", "/account/teams", map[string]any{"name": fmt.Sprintf("DEMO Operations %02d", i)}, 201)
		path := fmt.Sprintf("/account/teams/%d", team.Item.ID)
		s.api(admin, "POST", path+"/fund", map[string]any{"credits": 100, "idempotency_key": fmt.Sprintf("demo-admin-team-%d", i)}, 200)
		invite := s.api(admin, "POST", path+"/invites", map[string]any{}, 201)
		member := client()
		s.api(member, "POST", "/auth/login", map[string]any{"username": "demo_member", "password": Password}, 200)
		s.api(member, "POST", "/account/team-invites/accept", map[string]any{"code": invite.Code}, 200)
		s.logout(member)
	}
	for i := 1; i <= 9; i++ {
		if s.err != nil {
			return s.err
		}
		s.api(admin, "POST", "/admin/plans", map[string]any{"name": fmt.Sprintf("DEMO Bundle %02d", i), "credits": i * 2000, "price_cents": i * 1000, "currency": "CNY", "enabled": i%3 != 0}, 201)
	}
	for i := 1; i <= 58; i++ {
		if s.err != nil {
			return s.err
		}
		code := s.api(admin, "POST", "/admin/redemption-codes", map[string]any{"credits": 100 + i*10, "note": fmt.Sprintf("DEMO campaign %02d", i)}, 201)
		if i%2 == 0 {
			s.api(admin, "POST", "/account/redeem", map[string]any{"code": code.Code}, 200)
		}
	}
	for i := 1; i <= 56; i++ {
		if s.err != nil {
			return s.err
		}
		c := first
		var id int64
		if i != 1 {
			c = client()
			user := s.api(c, "POST", "/auth/register", map[string]any{"username": fmt.Sprintf("demo_user_%02d", i), "password": Password}, 201)
			id = user.User.ID
		}
		var token string
		for n := 1; n <= 3; n++ {
			t := s.api(c, "POST", "/account/tokens", map[string]any{"name": fmt.Sprintf("DEMO device %d", n)}, 201)
			if n == 1 {
				token = t.Token
			}
			if n == 3 {
				s.api(c, "DELETE", fmt.Sprintf("/account/tokens/%d", t.Item.ID), nil, 204)
			}
		}
		for n := 1; n <= 12; n++ {
			message := any(fmt.Sprintf("DEMO project %02d / run %02d", i, n))
			if n%4 == 0 {
				message = 17
			} // Real invalid input produces a refunded error, not a fabricated log.
			s.call(token, "echo", map[string]any{"message": message}, n%4 == 0)
		}
		if i%10 == 0 {
			s.api(admin, "PATCH", fmt.Sprintf("/admin/users/%d", id), map[string]any{"enabled": false}, 200)
		}
		s.logout(c)
	}
	return s.err
}
