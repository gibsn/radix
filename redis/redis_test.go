package redis

import (
	"flag"
	. "launchpad.net/gocheck"
	"testing"
	"time"
)

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) {
	TestingT(t)
}

//* Helpers
var rd *Client

// hashableTestType is a simple type implementing the
// Hashable interface.
type hashableTestType struct {
	a string
	b int64
	c bool
	d float64
}

// GetHash returns the fields as hash.
func (htt *hashableTestType) GetHash() Hash {
	h := NewHash()

	h.Set("hashable:field:a", htt.a)
	h.Set("hashable:field:b", htt.b)
	h.Set("hashable:field:c", htt.c)
	h.Set("hashable:field:d", htt.d)

	return h
}

// SetHash sets the fields from a hash.
func (htt *hashableTestType) SetHash(h Hash) {
	htt.a = h.String("hashable:field:a")
	htt.b = h.Int64("hashable:field:b")
	htt.c = h.Bool("hashable:field:c")
	htt.d = h.Float64("hashable:field:d")
}

func setUpTest(c *C) {
	rd = NewClient(Configuration{
		Database: 8,
		Address:  "127.0.0.1:6379"})

	// We use databases 8 and 9 for testing.
	rs := rd.MultiCommand(func(mc *MultiCommand) {
		mc.Command("flushdb") // flush db 8
		mc.Command("select", "9")
		mc.Command("flushdb")
		mc.Command("select", "8")
	})

	if !rs.OK() {
		c.Fatal("Setting up test databases failed.")
	}
}

func tearDownTest(c *C) {
	// Clean up our changes.
	rs := rd.MultiCommand(func(mc *MultiCommand) {
		mc.Command("select", "8")
		mc.Command("flushdb")
		mc.Command("select", "9")
		mc.Command("flushdb")
	})

	if !rs.OK() {
		c.Fatal("Cleaning up test databases failed.")
	}
}

//* Tests
type S struct{}
type Long struct{}

var long = flag.Bool("long", false, "Include blocking tests")

func init() {
	Suite(&S{})
	Suite(&Long{})
}

func (s *Long) SetUpSuite(c *C) {
	if !*long {
		c.Skip("-long not provided")
	}
}

func (s *S) SetUpTest(c *C) {
	setUpTest(c)
}

func (s *S) TearDownTest(c *C) {
	tearDownTest(c)
}

func (s *Long) SetUpTest(c *C) {
	setUpTest(c)
}

func (s *Long) TearDownTest(c *C) {
	tearDownTest(c)
}

// Test Select.
func (s *S) TestSelect(c *C) {
	rd.Select(9)
	c.Check(rd.configuration.Database, Equals, 9)
	rd.Command("set", "foo", "bar")

	rdA := NewClient(Configuration{Database: 9})
	c.Check(rdA.Command("get", "foo").String(), Equals, "bar")
}

// Test connection commands.
func (s *S) TestConnection(c *C) {
	c.Check(rd.Command("echo", "Hello, World!").String(), Equals, "Hello, World!")
	c.Check(rd.Command("ping").String(), Equals, "PONG")
}

// Test single return value commands.
func (s *S) TestSimpleValue(c *C) {
	// Simple value commands.
	rd.Command("set", "simple:string", "Hello,")
	rd.Command("append", "simple:string", " World!")
	c.Check(rd.Command("get", "simple:string").String(), Equals, "Hello, World!")

	rd.Command("set", "simple:int", 10)
	c.Check(rd.Command("incr", "simple:int").Int(), Equals, 11)

	rd.Command("set", "simple:float64", 47.11)
	c.Check(rd.Command("get", "simple:float64").Value().Float64(), Equals, 47.11)

	rd.Command("setbit", "simple:bit", 0, true)
	rd.Command("setbit", "simple:bit", 1, true)
	c.Check(rd.Command("getbit", "simple:bit", 0).Bool(), Equals, true)
	c.Check(rd.Command("getbit", "simple:bit", 1).Bool(), Equals, true)

	c.Check(rd.Command("get", "non:existing:key").OK(), Equals, false)
	c.Check(rd.Command("exists", "non:existing:key").Bool(), Equals, false)
	c.Check(rd.Command("setnx", "simple:nx", "Test").Bool(), Equals, true)
	c.Check(rd.Command("setnx", "simple:nx", "Test").Bool(), Equals, false)
}

// Test multi return value commands.
func (s *S) TestMultiple(c *C) {
	// Set values first.
	rd.Command("set", "multiple:a", "a")
	rd.Command("set", "multiple:b", "b")
	rd.Command("set", "multiple:c", "c")

	c.Check(
		rd.Command("mget", "multiple:a", "multiple:b", "multiple:c").Strings(),
		Equals,
		[]string{"a", "b", "c"})

	rd.Command("mset", hashableTestType{"multi", 4711, true, 3.141})
	if v := rd.Command("mget", "hashable:field:a", "hashable:field:c").Values(); len(v) != 2 {
	}
}

// Test hash accessing.
func (s *S) TestHash(c *C) {
	//* Single  return value commands.
	rd.Command("hset", "hash:bool", "true:1", 1)
	rd.Command("hset", "hash:bool", "true:2", true)
	rd.Command("hset", "hash:bool", "true:3", "T")
	rd.Command("hset", "hash:bool", "false:1", 0)
	rd.Command("hset", "hash:bool", "false:2", false)
	rd.Command("hset", "hash:bool", "false:3", "FALSE")
	c.Check(rd.Command("hget", "hash:bool", "true:1").Bool(), Equals, true)
	c.Check(rd.Command("hget", "hash:bool", "true:2").Bool(), Equals, true)
	c.Check(rd.Command("hget", "hash:bool", "true:3").Bool(), Equals, true)
	c.Check(rd.Command("hget", "hash:bool", "false:1").Bool(), Equals, false)
	c.Check(rd.Command("hget", "hash:bool", "false:2").Bool(), Equals, false)
	c.Check(rd.Command("hget", "hash:bool", "false:3").Bool(), Equals, false)

	ha := rd.Command("hgetall", "hash:bool").Hash()
	c.Assert(ha.Len(), Equals, 6)
	c.Check(ha.Bool("true:1"), Equals, true)
	c.Check(ha.Bool("true:2"), Equals, true)
	c.Check(ha.Bool("true:3"), Equals, true)
	c.Check(ha.Bool("false:1"), Equals, false)
	c.Check(ha.Bool("false:2"), Equals, false)
	c.Check(ha.Bool("false:3"), Equals, false)

	hb := hashableTestType{`foo "bar" yadda`, 4711, true, 8.15}
	rd.Command("hmset", "hashable", hb.GetHash())
	rd.Command("hincrby", "hashable", "hashable:field:b", 289)
	hb = hashableTestType{}
	hb.SetHash(rd.Command("hgetall", "hashable").Hash())
	c.Check(hb.a, Equals, `foo "bar" yadda`)
	c.Check(hb.b, Equals, int64(5000))
	c.Check(hb.c, Equals, true)
	c.Check(hb.d, Equals, 8.15)
}

// Test list commands.
func (s *S) TestList(c *C) {
	rd.Command("rpush", "list:a", "one")
	rd.Command("rpush", "list:a", "two")
	rd.Command("rpush", "list:a", "three")
	rd.Command("rpush", "list:a", "four")
	rd.Command("rpush", "list:a", "five")
	rd.Command("rpush", "list:a", "six")
	rd.Command("rpush", "list:a", "seven")
	rd.Command("rpush", "list:a", "eight")
	rd.Command("rpush", "list:a", "nine")
	c.Check(rd.Command("llen", "list:a").Int(), Equals, 9)
	c.Check(rd.Command("lpop", "list:a").String(), Equals, "one")

	vs := rd.Command("lrange", "list:a", 3, 6).Values()
	c.Assert(len(vs), Equals, 4)
	c.Check(vs[0].String(), Equals, "five")
	c.Check(vs[1].String(), Equals, "six")
	c.Check(vs[2].String(), Equals, "seven")
	c.Check(vs[3].String(), Equals, "eight")

	rd.Command("ltrim", "list:a", 0, 3)
	c.Check(rd.Command("llen", "list:a").Int(), Equals, 4)

	rd.Command("rpoplpush", "list:a", "list:b")
	c.Check(rd.Command("lindex", "list:b", 4711).OK(), Equals, false)
	c.Check(rd.Command("lindex", "list:b", 0).String(), Equals, "five")

	rd.Command("rpush", "list:c", 1)
	rd.Command("rpush", "list:c", 2)
	rd.Command("rpush", "list:c", 3)
	rd.Command("rpush", "list:c", 4)
	rd.Command("rpush", "list:c", 5)
	c.Check(rd.Command("lpop", "list:c").Int(), Equals, 1)
}

// Test set commands.
func (s *S) TestSets(c *C) {
	rd.Command("sadd", "set:a", 1)
	rd.Command("sadd", "set:a", 2)
	rd.Command("sadd", "set:a", 3)
	rd.Command("sadd", "set:a", 4)
	rd.Command("sadd", "set:a", 5)
	rd.Command("sadd", "set:a", 4)
	rd.Command("sadd", "set:a", 3)
	c.Check(rd.Command("scard", "set:a").Int(), Equals, 5)
	c.Check(rd.Command("sismember", "set:a", "4").Bool(), Equals, true)
}

// Test asynchronous commands.
func (s *S) TestAsync(c *C) {
	fut := rd.AsyncCommand("PING")
	rs := fut.ResultSet()
	c.Check(rs.String(), Equals, "PONG")
}

// Test complex commands.
func (s *S) TestComplex(c *C) {
	rsA := rd.Command("info")
	c.Check(rsA.Value().StringMap()["arch_bits"], NotNil)

	sliceIn := []string{"A", "B", "C", "D", "E"}
	rd.Command("set", "complex:slice", sliceIn)
	rsB := rd.Command("get", "complex:slice")
	sliceOut := rsB.Value().StringSlice()

	for i, s := range sliceOut {
		if sliceIn[i] != s {
			c.Errorf("Got '%v', expected '%v'!", s, sliceIn[i])
		}
	}

	mapIn := map[string]string{
		"A": "1",
		"B": "2",
		"C": "3",
		"D": "4",
		"E": "5",
	}

	rd.Command("set", "complex:map", mapIn)
	rsC := rd.Command("get", "complex:map")
	mapOut := rsC.Value().StringMap()

	for k, v := range mapOut {
		if mapIn[k] != v {
			c.Errorf("Got '%v', expected '%v'!", v, mapIn[k])
		}
	}
}

// Test multi-value commands.
func (s *S) TestMulti(c *C) {
	rd.Command("sadd", "multi:set", "one")
	rd.Command("sadd", "multi:set", "two")
	rd.Command("sadd", "multi:set", "three")

	c.Check(rd.Command("smembers", "multi:set").Len(), Equals, 3)
}

// Test transactions.
func (s *S) TestTransactions(c *C) {
	rsA := rd.MultiCommand(func(mc *MultiCommand) {
		mc.Command("set", "tx:a:string", "Hello, World!")
		mc.Command("get", "tx:a:string")
	})
	c.Check(rsA.ResultSetAt(1).String(), Equals, "Hello, World!")

	rsB := rd.MultiCommand(func(mc *MultiCommand) {
		mc.Command("set", "tx:b:string", "Hello, World!")
		mc.Command("get", "tx:b:string")
		mc.Discard()
		mc.Command("set", "tx:c:string", "Hello, Redis!")
		mc.Command("get", "tx:c:string")
	})
	c.Check(rsB.ResultSetAt(1).String(), Equals, "Hello, Redis!")

	// Failing transaction
	rsC := rd.MultiCommand(func(mc *MultiCommand) {
		mc.Command("get", "tx:c:string")
		mc.Command("set", "tx:c:string", "Hello, World!")
		mc.Command("get", "tx:c:string")
	})
	c.Check(rsC.ResultSetAt(2).String(), Not(Equals), "Hello, Redis!")
}

// Test subscribe.
func (s *S) TestSubscribe(c *C) {
	sub, numSubs, err := rd.Subscribe("subscribe:one", "subscribe:two")
	if err != nil {
		c.Errorf("Can't subscribe: '%v'!", err)
		return
	}
	c.Check(numSubs, Equals, 2)

	go func() {
		for sv := range sub.SubscriptionValueChan {
			if sv == nil {
				c.Log("Received nil!")
			} else {
				c.Logf("Published '%v' Channel '%v' Pattern '%v'", sv, sv.Channel, sv.ChannelPattern)
			}
		}
		c.Log("Subscription stopped!")
	}()
	c.Check(rd.Publish("subscribe:one", "1 Alpha"), Equals, 1)

	rd.Publish("subscribe:one", "1 Beta")
	rd.Publish("subscribe:one", "1 Gamma")
	rd.Publish("subscribe:two", "2 Alpha")
	rd.Publish("subscribe:two", "2 Beta")
	c.Log(sub.Unsubscribe("subscribe:two"))
	c.Log(sub.Unsubscribe("subscribe:one"))
	c.Check(rd.Publish("subscribe:two", "2 Gamma"), Equals, 0)

	sub.Subscribe("subscribe:*")
	rd.Publish("subscribe:one", "Pattern 1")
	rd.Publish("subscribe:two", "Pattern 2")
	sub.Stop()
}

//* Long tests

// Test pop.
func (s *Long) TestPop(c *C) {
	fooPush := func(rd *Client) {
		time.Sleep(time.Second)
		rd.Command("lpush", "pop:first", "foo")
	}

	// Set A: no database timeout.
	rdA := NewClient(Configuration{})

	go fooPush(rdA)

	rsAA := rdA.Command("blpop", "pop:first", 5)
	kv := rsAA.KeyValue()
	c.Check(kv.Value.String(), Equals, "foo")

	rsAB := rdA.Command("blpop", "pop:first", 1)
	c.Check(rsAB.OK(), Equals, true)

	// Set B: database with timeout.
	rdB := NewClient(Configuration{})

	rsBA := rdB.Command("blpop", "pop:first", 1)
	c.Check(rsBA.OK(), Equals, true)
}

// Test illegal databases.
func (s *Long) TestIllegalDatabases(c *C) {
	c.Log("Test selecting an illegal database...")
	rdA := NewClient(Configuration{Database: 4711})
	rsA := rdA.Command("ping")
	c.Check(rsA.OK(), Equals, true)

	c.Log("Test connecting to an illegal address...")
	rdB := NewClient(Configuration{Address: "192.168.100.100:12345"})
	rsB := rdB.Command("ping")
	c.Check(rsB.OK(), Equals, true)
}

// Test database killing with a long run.
func (s *Long) TestDatabaseKill(c *C) {
	rdA := NewClient(Configuration{PoolSize: 5})

	for i := 1; i < 120; i++ {
		if !rdA.Command("set", "long:run", i).OK() {
			c.Errorf("Long run failed!")
			return
		}
		time.Sleep(time.Second)
	}
}

//** Convenience method tests

//* Keys

func (s *S) TestDel(c *C) {
	rd.Command("set", "k1", "v1")
	rd.Command("set", "k2", "v2")
	c.Check(rd.Del("k1", "k2", "k3").Int(), Equals, 2)
}

func (s *S) TestExists(c *C) {
	rd.Command("set", "foo", "bar")
	c.Check(rd.Exists("foo").Bool(), Equals, true)
	c.Check(rd.Exists("bar").Bool(), Equals, false)
}

func (s *Long) TestExpire(c *C) {
	c.Check(rd.Expire("non-existent-key", 10).Bool(), Equals, false)

	rd.Command("set", "foo", "bar")
	c.Check(rd.Expire("foo", 5).Bool(), Equals, true)
	c.Check(rd.Command("ttl", "foo").Int(), Not(Equals), -1)
	time.Sleep(10 * time.Second)
	c.Check(rd.Command("exists", "foo").Bool(), Equals, false)
}

func (s *Long) TestExpireat(c *C) {
	rd.Command("set", "foo", "bar")
	rd.Expireat("foo", time.Now().Unix()+5)
	c.Check(rd.Command("ttl", "foo").Int(), Not(Equals), -1)
	time.Sleep(10 * time.Second)
	c.Check(rd.Command("exists", "foo").Bool(), Equals, false)
}

func (s *S) TestKeys(c *C) {
	expected := []string{"one", "two", "four"}

	rd.Command("set", "one", "1")
	rd.Command("set", "two", "2")
	rd.Command("set", "three", "3")
	rd.Command("set", "four", "4")
	rs := rd.Keys("*o*")
	c.Check(rs.OK(), Equals, true)

	for _, v1 := range rs.Strings() {
		missing := true
		for _, v2 := range expected {
			if v1 == v2 {
				missing = false
			}
		}

		if missing {
			c.Errorf("%s is missing from the expected keys!", v1)
		}
	}
}

func (s *S) TestMove(c *C) {
	rd.Command("set", "foo", "bar")
	rd.Move("foo", 9)
	c.Check(rd.Command("exists", "foo").Bool(), Equals, false)
	rd.Select(9)
	c.Check(rd.Command("get", "foo").String(), Equals, "bar")
}

func (s *S) TestObject(c *C) {
	// Not sure what to actually test here...
	rd.Command("set", "foo", "bar")
	c.Check(rd.Object("idletime", "foo").Int(), FitsTypeOf, 0)
}

func (s *S) TestPersist(c *C) {
	rd.Command("set", "foo", "bar")
	rd.Command("expire", "foo", 100)
	c.Check(rd.Persist("foo").Bool(), Equals, true)
	c.Check(rd.Command("ttl", "foo").Int(), Equals, -1)
}

func (s *S) TestRandomkey(c *C) {
	rd.Command("set", "foo", "bar")
	c.Check(rd.Randomkey().String(), Equals, "foo")
}

func (s *S) TestRename(c *C) {
	rd.Command("set", "foo", "bar")
	c.Check(rd.Rename("foo", "zot").OK(), Equals, true)
	c.Check(rd.Command("get", "zot").String(), Equals, "bar")
}

func (s *S) TestRenamenx(c *C) {
	rd.Command("set", "k1", "v1")
	rd.Command("set", "k2", "v2")
	c.Check(rd.Renamenx("k1", "k2").Bool(), Equals, false)
	c.Check(rd.Command("get", "k2").String(), Equals, "v2")
}

func (s *S) TestSort(c *C) {
	rd.Command("lpush", "foo", 4)
	rd.Command("lpush", "foo", 2)
	rd.Command("lpush", "foo", 6)
	rd.Command("lpush", "foo", 0)
	rd.Command("lpush", "foo", 9)
	c.Check(rd.Sort("foo").Ints(), Equals, []int{0, 2, 4, 6, 9})
}

func (s *S) TestTTL(c *C) {
	rd.Command("set", "foo", "bar")
	rd.Command("expire", "foo", 100)
	c.Check(rd.TTL("foo").Int(), Not(Equals), -1)
}

func (s *S) TestType(c *C) {
	rd.Command("set", "foo", "bar")
	c.Check(rd.Type("foo").String(), Equals, "string")
}

//* Strings

func (s *S) TestAppend(c *C) {
	c.Check(rd.Append("foo", "bar").Int(), Equals, 3)
	c.Check(rd.Append("foo", "zot").Int(), Equals, 6)
	c.Check(rd.Command("get", "foo").String(), Equals, "barzot")

	c.Check(rd.Append("foo2", 123).Int(), Equals, 3)
	c.Check(rd.Append("foo2", 456).Int(), Equals, 6)
	c.Check(rd.Command("get", "foo2").Int(), Equals, 123456)
}

func (s *S) TestDecr(c *C) {
	rd.Command("set", "foo", 10)
	c.Check(rd.Decr("foo").Int(), Equals, 9)
	rd.Command("set", "foo2", "bar")
	c.Check(rd.Decr("foo2").OK(), Equals, false)
}

func (s *S) TestDecrby(c *C) {
	rd.Command("set", "foo", 10)
	c.Check(rd.Decrby("foo", 5).Int(), Equals, 5)
}

func (s *S) TestGet(c *C) {
	c.Check(rd.Get("non:existing:key").OK(), Equals, false)
	rd.Command("set", "foo", "bar")
	c.Check(rd.Get("foo").String(), Equals, "bar")
}

func (s *S) TestGetbit(c *C) {
	rd.Command("setbit", "foo", 2, 1)
	c.Check(rd.Getbit("foo", 2).Bool(), Equals, true)
}

func (s *S) TestGetrange(c *C) {
	rd.Command("set", "foo", "this is a string")
	c.Check(rd.Getrange("foo", 0, 3).String(), Equals, "this")
}

func (s *S) TestGetset(c *C) {
	rd.Command("set", "foo", "hello")
	c.Check(rd.Getset("foo", "world").String(), Equals, "hello")
	c.Check(rd.Command("get", "foo").String(), Equals, "world")
}

func (s *S) TestIncr(c *C) {
	rd.Command("set", "foo", 10)
	c.Check(rd.Incr("foo").Int(), Equals, 11)
	c.Check(rd.Command("get", "foo").Int(), Equals, 11)
}

func (s *S) TestIncrby(c *C) {
	rd.Command("set", "foo", 10)
	c.Check(rd.Incrby("foo", 5).Int(), Equals, 15)
	c.Check(rd.Command("get", "foo").Int(), Equals, 15)
}

func (s *S) TestMget(c *C) {
	rd.Command("set", "foo", "hello")
	rd.Command("set", "bar", "world")
	c.Check(
		rd.Mget("foo", "bar", "nonexisting:key").Strings(),
		Equals,
		[]string{"hello", "world", ""})
}

func (s *S) TestMset(c *C) {
	c.Check(rd.Mset("key1", "val1", "key2", "val2").OK(), Equals, true)
	c.Check(rd.Command("get", "key1").String(), Equals, "val1")
	c.Check(rd.Command("get", "key2").String(), Equals, "val2")
}

func (s *S) TestMsetnx(c *C) {
	c.Check(rd.Msetnx("key1", "val1", "key2", "val2").Bool(), Equals, true)
	c.Check(rd.Msetnx("key2", "val2", "key3", "val3").Bool(), Equals, false)
	c.Check(rd.Command("get", "key1").String(), Equals, "val1")
	c.Check(rd.Command("get", "key2").String(), Equals, "val2")
	c.Check(rd.Command("get", "key3").String(), Equals, "")
}

func (s *S) TestSet(c *C) {
	c.Check(rd.Set("foo", "bar").OK(), Equals, true)
	c.Check(rd.Command("get", "foo").String(), Equals, "bar")
}

func (s *S) TestSetbit(c *C) {
	c.Check(rd.Setbit("foo", 7, true).Bool(), Equals, false)
	c.Check(rd.Setbit("foo", 7, false).Bool(), Equals, true)
	c.Check(rd.Command("get", "foo").String(), Equals, "\x00")
}

func (s *S) TestSetex(c *C) {
	c.Check(rd.Setex("foo", 100, "bar").OK(), Equals, true)
	c.Check(rd.Command("ttl", "foo").Int(), Not(Equals), 0)
	c.Check(rd.Command("get", "foo").String(), Equals, "bar")
}

func (s *S) TestSetnx(c *C) {
	c.Check(rd.Setnx("foo", "bar").Bool(), Equals, true)
	c.Check(rd.Setnx("foo", "bar").Bool(), Equals, false)
	c.Check(rd.Command("get", "foo").String(), Equals, "bar")
}

func (s *S) TestSetrange(c *C) {
	rd.Command("set", "foo", "hello world")
	c.Check(rd.Setrange("foo", 6, "redis").Int(), Equals, 11)
	c.Check(rd.Command("get", "foo").String(), Equals, "hello redis")
}

func (s *S) TestStrlen(c *C) {
	rd.Command("set", "foo", "bar")
	c.Check(rd.Strlen("foo").Int(), Equals, 3)
	c.Check(rd.Strlen("zot").Int(), Equals, 0)
}

//* Hashes
/*
func (s *S) TestHdel(c *C) {
	rd.Command("hset", "myset", "k1", "v1")
	rd.Command("hset", "myset", "k2", "v2")
	rd.Command("hset", "myset", "k3", "v3")
	rd.Hdel("myset", "k1", "k2")
	c.Check(rd.Command("hgetall", "myset").Value(), Equals, 3)
}
*/
