package netutils

import (
	"bytes"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"go.n16f.net/bcl"
)

type RateCounterCfg struct {
	Limit  int64
	Period int64 // milliseconds
}

type RateCounter struct {
	Start  int64 // UNIX millisecond timestamp
	Limit  int64
	Period int64 // milliseconds
	Count  int64
}

func NewRateCounter(cfg *RateCounterCfg) *RateCounter {
	rc := RateCounter{
		Limit:  cfg.Limit,
		Period: cfg.Period,
	}

	if cfg.Period > 0 {
		now := time.Now().UnixMilli()
		rc.Start = now - now%cfg.Period
	}

	return &rc
}

func (c *RateCounter) String() string {
	return fmt.Sprintf("RateCounter{%s %.0fs %d/%d}",
		time.Unix(c.Start/1000, c.Start%1000).UTC().Format(time.RFC3339),
		float64(c.Period)/1000.0, c.Count, c.Limit)
}

func (c *RateCounter) Update(n int, t time.Time) bool {
	if c.Period == 0 {
		// "Empty" counters (configured with a limit set to "none") always pass
		return true
	}

	now := t.UnixMilli()

	if now < c.Start {
		return false
	}

	if now-c.Start > c.Period {
		c.Start = now - now%c.Period
		c.Count = 0
	}

	n64 := int64(n)

	if c.Count+n64 > c.Limit {
		return false
	}

	c.Count += n64
	return true
}

type RateLimiterIPv4NetType int

func (t *RateLimiterIPv4NetType) Parse(s string) error {
	length, err := parseRateLimiterNetType(s, 32)
	if err != nil {
		return err
	}

	*t = RateLimiterIPv4NetType(length)
	return nil
}

func (t *RateLimiterIPv4NetType) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if err := t.Parse(s); err != nil {
		return fmt.Errorf("invalid IPv4 network type: %w", err)
	}

	return nil
}

type RateLimiterIPv6NetType int

func (t *RateLimiterIPv6NetType) Parse(s string) error {
	length, err := parseRateLimiterNetType(s, 128)
	if err != nil {
		return err
	}

	*t = RateLimiterIPv6NetType(length)
	return nil
}

func (t *RateLimiterIPv6NetType) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if err := t.Parse(s); err != nil {
		return fmt.Errorf("invalid IPv6 network type: %w", err)
	}

	return nil
}

func parseRateLimiterNetType(s string, max int) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty network type")
	}

	if s[0] != '/' {
		return 0, fmt.Errorf("invalid network type: " +
			"value must start with a '/' character")
	}

	i64, err := strconv.ParseInt(s[1:], 10, 64)
	if err != nil || i64 < 0 {
		return 0, fmt.Errorf("invalid network type: invalid prefix %q", s[1:])
	}

	if i64 < 1 || i64 > int64(max) {
		return 0, fmt.Errorf("invalid network type: "+
			"prefix length must be between 1 and %d", max)
	}

	return int(i64), nil
}

type AddressRateLimiter struct {
	NetAddress  net.IPNet
	RateLimiter *RateCounter
}

type AddressRateLimiters []AddressRateLimiter

func (rls AddressRateLimiters) Sort() {
	slices.SortFunc(rls,
		func(addrRL1, addrRL2 AddressRateLimiter) int {
			addr1 := addrRL1.NetAddress
			addr2 := addrRL2.NetAddress

			if len(addr1.IP) < len(addr2.IP) {
				return -1
			}

			if len(addr1.IP) > len(addr2.IP) {
				return 1
			}

			prefixLength1, _ := addr1.Mask.Size()
			prefixLength2, _ := addr2.Mask.Size()

			if prefixLength1 > prefixLength2 {
				return -1
			}

			if prefixLength1 < prefixLength2 {
				return 1
			}

			return bytes.Compare([]byte(addr1.String()),
				[]byte(addr2.String()))
		})
}

type RateCounterTable map[string]*RateCounter

type RateLimiterCfg struct {
	Global         *RateCounterCfg
	PerIPv4Address map[RateLimiterIPv4NetType]*RateCounterCfg
	PerIPv6Address map[RateLimiterIPv6NetType]*RateCounterCfg
	Addresses      map[*IPNetAddr]*RateCounterCfg
}

func (cfg *RateLimiterCfg) ReadBCLElement(elt *bcl.Element) error {
	cfg.PerIPv4Address =
		make(map[RateLimiterIPv4NetType]*RateCounterCfg)
	cfg.PerIPv6Address =
		make(map[RateLimiterIPv6NetType]*RateCounterCfg)
	cfg.Addresses = make(map[*IPNetAddr]*RateCounterCfg)

	readRateLimit := func(pCfg **RateCounterCfg, entry *bcl.Element, firstValue int) bool {
		var cfg RateCounterCfg

		if entry.NbValues() == firstValue+1 {
			entry.CheckValueOneOf(firstValue, "none")
		} else {
			if !entry.CheckNbValues(firstValue + 2) {
				return false
			}

			if !entry.Value(firstValue, &cfg.Limit) {
				return false
			}

			var period int64
			if !entry.Value(firstValue+1, &period) {
				return false
			}
			cfg.Period = period * 1000
		}

		*pCfg = &cfg

		return true
	}

	if elt.IsBlock() {
		if entry := elt.FindEntry("global"); entry != nil {
			readRateLimit(&cfg.Global, entry, 0)
		}

		if entry := elt.FindEntry("per_address"); entry != nil {
			var rcCfg *RateCounterCfg
			readRateLimit(&rcCfg, entry, 0)

			if rcCfg != nil {
				cfg.PerIPv4Address[RateLimiterIPv4NetType(32)] = rcCfg
				cfg.PerIPv6Address[RateLimiterIPv6NetType(48)] = rcCfg
			}
		}

		if entry := elt.FindEntry("per_ipv4_address"); entry != nil {
			var rcCfg *RateCounterCfg
			readRateLimit(&rcCfg, entry, 0)

			if rcCfg != nil {
				cfg.PerIPv4Address[RateLimiterIPv4NetType(32)] = rcCfg
			}
		}

		for _, entry := range elt.FindEntries("per_ipv4_network") {
			var netType RateLimiterIPv4NetType
			entry.Value(0, &netType)

			var rcCfg *RateCounterCfg
			readRateLimit(&rcCfg, entry, 1)

			if rcCfg != nil {
				cfg.PerIPv4Address[netType] = rcCfg
			}
		}

		for _, entry := range elt.FindEntries("per_ipv6_network") {
			var netType RateLimiterIPv6NetType
			entry.Value(0, &netType)

			var rcCfg *RateCounterCfg
			readRateLimit(&rcCfg, entry, 1)

			if rcCfg != nil {
				cfg.PerIPv6Address[netType] = rcCfg
			}
		}

		for _, entry := range elt.FindEntries("address") {
			var addr IPAddr
			entry.Value(0, &addr)

			var rcCfg *RateCounterCfg
			readRateLimit(&rcCfg, entry, 1)

			if rcCfg != nil {
				netAddr := IPNetAddr(addr)
				cfg.Addresses[&netAddr] = rcCfg
			}
		}

		for _, entry := range elt.FindEntries("network") {
			var addr IPNetAddr
			entry.Value(0, &addr)

			var rcCfg *RateCounterCfg
			readRateLimit(&rcCfg, entry, 1)

			if rcCfg != nil {
				cfg.Addresses[&addr] = rcCfg
			}
		}
	} else {
		elt.CheckValueOneOf(0, "none")
	}

	return nil
}

func (cfg *RateLimiterCfg) IsEmpty() bool {
	return cfg.Global == nil &&
		len(cfg.PerIPv4Address) == 0 &&
		len(cfg.PerIPv6Address) == 0 &&
		len(cfg.Addresses) == 0
}

type RateLimiter struct {
	Cfg *RateLimiterCfg

	global         *RateCounter
	perIPv4Address map[RateLimiterIPv4NetType]RateCounterTable
	perIPv6Address map[RateLimiterIPv6NetType]RateCounterTable
	addresses      AddressRateLimiters

	mutex sync.Mutex
}

func NewRateLimiter(cfg *RateLimiterCfg) *RateLimiter {
	rl := RateLimiter{
		Cfg: cfg,
	}

	if cfg.Global != nil {
		rl.global = NewRateCounter(cfg.Global)
	}

	if len(cfg.PerIPv4Address) > 0 {
		rl.perIPv4Address = make(map[RateLimiterIPv4NetType]RateCounterTable)
		for netType := range cfg.PerIPv4Address {
			rl.perIPv4Address[netType] = make(RateCounterTable)
		}
	}

	if len(cfg.PerIPv6Address) > 0 {
		rl.perIPv6Address = make(map[RateLimiterIPv6NetType]RateCounterTable)
		for netType := range cfg.PerIPv6Address {
			rl.perIPv6Address[netType] = make(RateCounterTable)
		}
	}

	if len(cfg.Addresses) > 0 {
		for addr, rcCfg := range cfg.Addresses {
			rl.addresses = append(rl.addresses, AddressRateLimiter{
				NetAddress:  net.IPNet(*addr),
				RateLimiter: NewRateCounter(rcCfg),
			})
		}

		rl.addresses.Sort()
	}

	return &rl
}

func (rl *RateLimiter) GC() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	gc := func(table RateCounterTable) {
		for addr, counter := range table {
			if counter.Count == 0 {
				delete(table, addr)
			}
		}
	}

	for _, table := range rl.perIPv4Address {
		gc(table)
	}

	for _, table := range rl.perIPv6Address {
		gc(table)
	}
}

func (rl *RateLimiter) Update(n int, addr net.IP, now time.Time) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	pass := true

	if rl.global != nil {
		if rl.global.Update(n, now) == false {
			pass = false
		}
	}

	if rl.perIPv4Address != nil && len(addr) == net.IPv4len {
		for prefixLength, table := range rl.perIPv4Address {
			mask := net.CIDRMask(int(prefixLength), 32)
			netAddr := net.IPNet{
				IP:   addr.Mask(mask),
				Mask: mask,
			}
			netAddrString := netAddr.String()

			rc, found := table[netAddrString]
			if !found {
				rc = NewRateCounter(rl.Cfg.PerIPv4Address[prefixLength])
				table[netAddrString] = rc
			}

			if rc.Update(n, now) == false {
				pass = false
			}
		}
	}

	if rl.perIPv6Address != nil && len(addr) == net.IPv6len {
		for prefixLength, table := range rl.perIPv6Address {
			mask := net.CIDRMask(int(prefixLength), 128)
			netAddr := net.IPNet{
				IP:   addr.Mask(mask),
				Mask: mask,
			}
			netAddrString := netAddr.String()

			rc, found := table[netAddrString]
			if !found {
				rc = NewRateCounter(rl.Cfg.PerIPv6Address[prefixLength])
				table[netAddrString] = rc
			}

			if rc.Update(n, now) == false {
				pass = false
			}
		}
	}

	addressPass := false

	if rl.addresses != nil {
		for _, addrRL := range rl.addresses {
			if addrRL.NetAddress.Contains(addr) {
				if addrRL.RateLimiter.Update(n, now) == true {
					addressPass = true
					break
				}
			}
		}

		// Specific address limits override general limits
		if !addressPass {
			return false
		}
	}

	return pass || addressPass
}
