package main

/*
#cgo pkg-config: libiptc
#cgo pkg-config: xtables
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <error.h>
#include <libiptc/libiptc.h>
#include <libiptc/libip6tc.h>
#include <xtables.h>


char buf[150];

void print_match_ipv4(struct xt_entry_match *m,struct ipt_ip *ip, int numeric, char *buf) {
	int fds[2];
  	pipe(fds);

  	if(!fork()) { //child element
  		close(fds[0]);
  		dup2(fds[1], STDOUT_FILENO);
		printf("-m ");
		xtables_init();
		xtables_set_nfproto(NFPROTO_IPV4);
		const struct xtables_match *match = xtables_find_match(m->u.user.name, XTF_LOAD_MUST_SUCCEED, NULL);
	    if (match) {
	        if (match->print)
	            match->print(ip, m, numeric);
	        else
	            printf("%s ", match->name);
	    } else {
	        if (m->u.user.name[0])
	            printf("UNKNOWN match `%s' ", m->u.user.name);
	    }
	    exit(1);
	}
	else {
		close(fds[1]);
  		read(fds[0],buf,150);
	}
    
}
int match_iterate_wrapper_ipv4 (struct ipt_entry *e, unsigned int i) {
	memset(buf, 0, 150);
	struct xt_entry_match *m;
    m = (void *)e + i;
    i += m->u.match_size;
    print_match_ipv4(m , &e->ip, 0x0008, buf);
	return i;
}
int getSizeIptEntry() {
	return ((int) sizeof(struct ipt_entry)); 
}


void print_match_ipv6(struct xt_entry_match *m,struct ip6t_ip6 *ip, int numeric, char *buf) {
	int fds[2];
  	pipe(fds);

  	if(!fork()) { //child element
  		close(fds[0]);
  		dup2(fds[1], STDOUT_FILENO);
		printf("-m ");
		xtables_init();
		xtables_set_nfproto(NFPROTO_IPV6);
	    const struct xtables_match *match = xtables_find_match(m->u.user.name, XTF_LOAD_MUST_SUCCEED, NULL);
	    if (match) {
	        if (match->print)
	            match->print(ip, m, numeric);
	        else
	            printf("%s ", match->name);
	    } else {
	        if (m->u.user.name[0])
	            printf("UNKNOWN match `%s' ", m->u.user.name);
	    }
	    exit(1);
	}
	else {
		close(fds[1]);
  		read(fds[0],buf,150);
	}
    
}
int match_iterate_wrapper_ipv6 (struct ip6t_entry *e, unsigned int i) {
	memset(buf, 0, 150);
	struct xt_entry_match *m;
    m = (void *)e + i;
    i += m->u.match_size;
    print_match_ipv6(m , &e->ipv6, 0x0008, buf);
	return i;
}
int getSizeIpt6Entry() {
	return ((int) sizeof(struct ip6t_entry)); 
}

*/
import "C"
import "errors"
import "fmt"
import "net"
import "bytes"
import "os"
import "unsafe"
import "encoding/json"
//import "reflect"

/**
 * Declaration of structures and interfaces
 *
 *
 *
 */

//
type IPT struct {
	h *C.struct_xtc_handle
}

//
type IP6T struct {
	h *C.struct_xtc_handle
}

//
type Not bool

//
type Counter struct {
	Packets uint64
	Bytes   uint64
}

//
type Rule struct {
	Src    *net.IPNet
	Dest   *net.IPNet
	InDev  string
	OutDev string
	Not    struct {
		Src    Not
		Dest   Not
		InDev  Not
		OutDev Not
	}
	Matches []string
	Target string
	Counter
}

var (
	ErrorCustomChain = errors.New("Custom chains dont have counters defined :/")
)

//
type IPTi interface {
	IsBuiltinChain(string) bool
	Chains() []string
	Close() error
	Counter(chain string) (Counter, error)
	Rules(chain string) []*Rule
	Zero(chain string) error
}

// Make a snapshot of the current iptables rules
func NewIPT(table string) (IPTi, error) {
	cname := C.CString(table)
	defer C.free(unsafe.Pointer(cname))
	s := new(IPT)
	h, err := C.iptc_init(cname)

	if err != nil {
		return nil, err
	}
	s.h = h
	return s, nil
}

func (s *IPT) Chains() []string {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	chains := []string{}

	for c := C.iptc_first_chain(s.h); c != nil; c = C.iptc_next_chain(s.h) {
		chains = append(chains, C.GoString(c))
	}

	return chains
}

func (s *IPT) IsBuiltinChain(chain string) bool {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	return int(C.iptc_builtin(cname, s.h)) != 0
}

func (s *IPT) Counter(chain string) (Counter, error) {
	var c Counter
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	if !s.IsBuiltinChain(chain) {
		return c, ErrorCustomChain
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	count := new(C.struct_xt_counters)
	_, err := C.iptc_get_policy(cname, count, s.h)

	if err != nil {
		return c, err
	}
	c.Packets = uint64(count.pcnt)
	c.Bytes = uint64(count.bcnt)

	return c, nil

}

func (s *IPT) Rules(chain string) []*Rule {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	rules := make([]*Rule, 0)

	for r := C.iptc_first_rule(cname, s.h); r != nil; r = C.iptc_next_rule(r, s.h) {
		c := new(Rule)

		// read counters
		c.Packets = uint64(r.counters.pcnt)
		c.Bytes = uint64(r.counters.bcnt)

		// read network interfaces
		c.InDev = C.GoString(&r.ip.iniface[0])
		c.OutDev = C.GoString(&r.ip.outiface[0])
		if r.ip.invflags&C.IPT_INV_VIA_IN != 0 {
			c.Not.InDev = true
		}
		if r.ip.invflags&C.IPT_INV_VIA_OUT != 0 {
			c.Not.OutDev = true
		}

		// read source ip and mask
		src := uint32(r.ip.src.s_addr)
		c.Src = new(net.IPNet)
		c.Src.IP = net.IPv4(byte(src&0xff),
							byte((src>>8)&0xff),
							byte((src>>16)&0xff),
							byte((src>>24)&0xff))
		mask := uint32(r.ip.smsk.s_addr)
		c.Src.Mask = net.IPv4Mask(byte(mask&0xff),
								byte((mask>>8)&0xff),
								byte((mask>>16)&0xff),
								byte((mask>>24)&0xff))
		if r.ip.invflags&C.IPT_INV_SRCIP != 0 {
			c.Not.Src = true
		}

		// read destination ip and mask
		dest := uint32(r.ip.dst.s_addr)
		c.Dest = new(net.IPNet)
		c.Dest.IP = net.IPv4(byte(dest&0xff),
							byte((dest>>8)&0xff),
							byte((dest>>16)&0xff),
							byte((dest>>24)&0xff))
		mask = uint32(r.ip.dmsk.s_addr)
		c.Dest.Mask = net.IPv4Mask(byte(mask&0xff),
								byte((mask>>8)&0xff),
								byte((mask>>16)&0xff),
								byte((mask>>24)&0xff))
		if r.ip.invflags&C.IPT_INV_DSTIP != 0 {
			c.Not.Dest = true
		}
		//read match 

		target_offset := int(r.target_offset)
		if(target_offset > 0) {
			for i := uint64(C.getSizeIptEntry()); int(i) < target_offset ;{
				i = uint64 (C.match_iterate_wrapper_ipv4(r, C.uint(i)))
				match := C.GoString(&C.buf[0])
				c.Matches = append(c.Matches, match)
			}
		}

		// read target
		target := C.iptc_get_target(r, s.h)
		if target != nil {
			c.Target = C.GoString(target)
		}

		rules = append(rules, c)
	}

	return rules
}

func (s *IPT) Zero(chain string) error {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	ret, err := C.iptc_zero_entries(cname, s.h)

	if err != nil || ret != 1 {
		return err
	}

	return nil
}

// commit and free resources
func (s *IPT) Close() error {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	ret, err := C.iptc_commit(s.h)
	if err != nil || ret != 1 {
		return err
	}

	C.iptc_free(s.h)
	s.h = nil

	return nil
}

func NewIP6T(table string) (IPTi, error) {
	cname := C.CString(table)
	defer C.free(unsafe.Pointer(cname))
	s := new(IP6T)
	h, err := C.ip6tc_init(cname)

	if err != nil {
		return nil, err
	}
	s.h = h
	return s, nil
}

func (s *IP6T) Chains() []string {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	chains := []string{}

	for c := C.ip6tc_first_chain(s.h); c != nil; c = C.ip6tc_next_chain(s.h) {
		chains = append(chains, C.GoString(c))
	}

	return chains
}

func (s *IP6T) IsBuiltinChain(chain string) bool {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	return int(C.ip6tc_builtin(cname, s.h)) != 0
}

func (s *IP6T) Counter(chain string) (Counter, error) {
	var c Counter
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	if !s.IsBuiltinChain(chain) {
		return c, ErrorCustomChain
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	count := new(C.struct_xt_counters)
	_, err := C.ip6tc_get_policy(cname, count, s.h)

	if err != nil {
		return c, err
	}

	c.Packets = uint64(count.pcnt)
	c.Bytes = uint64(count.bcnt)

	return c, nil
}

func (s *IP6T) Rules(chain string) []*Rule {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	rules := make([]*Rule, 0)

	for r := C.ip6tc_first_rule(cname, s.h); r != nil; r = C.ip6tc_next_rule(r, s.h) {
		c := new(Rule)

		// read counters
		c.Packets = uint64(r.counters.pcnt)
		c.Bytes = uint64(r.counters.bcnt)

		// read network interfaces
		c.InDev = C.GoString(&r.ipv6.iniface[0])
		c.OutDev = C.GoString(&r.ipv6.outiface[0])
		if r.ipv6.invflags&C.IP6T_INV_VIA_IN != 0 {
			c.Not.InDev = true
		}
		if r.ipv6.invflags&C.IP6T_INV_VIA_OUT != 0 {
			c.Not.OutDev = true
		}

		// read source ip and mask
		src := r.ipv6.src.__in6_u
		c.Src = new(net.IPNet)
		c.Src.IP = net.IP{src[0], src[1], src[2], src[3],
			src[4], src[5], src[6], src[7],
			src[8], src[9], src[10], src[11],
			src[12], src[13], src[14], src[15]}
		mask := r.ipv6.smsk.__in6_u
		c.Src.Mask = net.IPMask{mask[0], mask[1], mask[2], mask[3],
			mask[4], mask[5], mask[6], mask[7],
			mask[8], mask[9], mask[10], mask[11],
			mask[12], mask[13], mask[14], mask[15]}
		if r.ipv6.invflags&C.IP6T_INV_SRCIP != 0 {
			c.Not.Src = true
		}

		// read destination ip and mask
		dest := r.ipv6.dst.__in6_u
		c.Dest = new(net.IPNet)
		c.Dest.IP = net.IP{dest[0], dest[1], dest[2], dest[3],
			dest[4], dest[5], dest[6], dest[7],
			dest[8], dest[9], dest[10], dest[11],
			dest[12], dest[13], dest[14], dest[15]}
		mask = r.ipv6.dmsk.__in6_u
		c.Dest.Mask = net.IPMask{mask[0], mask[1], mask[2], mask[3],
			mask[4], mask[5], mask[6], mask[7],
			mask[8], mask[9], mask[10], mask[11],
			mask[12], mask[13], mask[14], mask[15]}
		if r.ipv6.invflags&C.IP6T_INV_DSTIP != 0 {
			c.Not.Dest = true
		}

		//read matches
		target_offset := int(r.target_offset)
		if(target_offset > 0) {
			for i := uint64(C.getSizeIpt6Entry()); int(i) < target_offset ;{
				i = uint64 (C.match_iterate_wrapper_ipv6(r, C.uint(i)))
				match := C.GoString(&C.buf[0])
				c.Matches = append(c.Matches, match)
			}
		}

		// read target
		target := C.ip6tc_get_target(r, s.h)
		if target != nil {
			c.Target = C.GoString(target)
		}

		rules = append(rules, c)
	}

	return rules
}

func (s *IP6T) Zero(chain string) error {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	cname := C.CString(chain)
	defer C.free(unsafe.Pointer(cname))

	ret, err := C.ip6tc_zero_entries(cname, s.h)

	if err != nil || ret != 1 {
		return err
	}

	return nil
}

// commit and free resources
func (s *IP6T) Close() error {
	if s.h == nil {
		panic("trying to use libiptc handle after Close()")
	}

	if s.h == nil {
		return nil
	}

	ret, err := C.ip6tc_commit(s.h)
	if err != nil || ret != 1 {
		return err
	}

	C.ip6tc_free(s.h)
	s.h = nil

	return nil
}

func (r Rule) String() string {
	return fmt.Sprintf("in: %s%s, out: %s%s, %s%s -> %s%s -> %s: %s : %d packets, %d bytes",
		r.Not.InDev, r.InDev,
		r.Not.OutDev, r.OutDev,
		r.Not.Src, r.Src,
		r.Not.Dest, r.Dest,
		r.Matches,
		r.Target,
		r.Packets, r.Bytes)
}


func (n Not) String() string {
	if n {
		return "!"
	}
	return " "
}

func print_rule4() {
	ipt, err := NewIPT("filter")

	if (err != nil) {
		panic("Error occured initializing filter table")
	}

	chains := ipt.Chains()
	for _,chain := range chains {
		if(!ipt.IsBuiltinChain(chain)) {
			continue;
		}

		rules := ipt.Rules(chain)

		for index , rule := range rules {
			fmt.Printf ("\n Index: %d, Chain: %s, Rule: %s \n", index, chain, rule)
		}

		byt, _ := json.Marshal(rules)

		
		var out bytes.Buffer
		json.Indent(&out, byt, "=", "\t")
		out.WriteTo(os.Stdout)
		

	}
	ipt.Close()	
}

func print_rule6() {
	ip6t, err := NewIP6T("filter")

	if (err != nil) {
		panic("Error occured initializing filter table")
	}

	chains := ip6t.Chains()
	for _,chain := range chains {
		if(!ip6t.IsBuiltinChain(chain)) {
			continue;
		}

		rules := ip6t.Rules(chain)

		for index , rule := range rules {
			fmt.Printf ("\n Index: %d, Chain: %s, Rule: %s \n", index, chain, rule)
		}

		byt, _ := json.Marshal(rules)

		
		var out bytes.Buffer
		json.Indent(&out, byt, "=", "\t")
		out.WriteTo(os.Stdout)
		

	}
	ip6t.Close()	
}

func main() {

	fmt.Println("\n----Iptables Rules -----\n");
	print_rule4();
	

	fmt.Println("\n----Ip6tables Rules -----\n");
	print_rule6();
	

}
