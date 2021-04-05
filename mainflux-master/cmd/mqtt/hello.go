package main

import (
	"fmt"
	"github.com/go-redis/redis"
)

func main() {
	rdb := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName: "mymaster",
		SentinelAddrs: []string{
			"10.38.23.110:26379",
			"10.38.23.111:26379",
			"10.38.23.112:26379",
		},
	})
	//rdb.Ping()
	rdb.HSet("88171961786836607","ID","abe21171-aea2-49ba-98b2-a520e2e62649")
	rdb.HSet("88171961786836607","KEY","42d47493-13b7-4df1-9547-046b024cbbb9")

	var value = rdb.HGet("88171961786836607","ID")
	var value2 = rdb.HGet("88171961786836607","KEY")
	//rdb.HGetAll("88171961786836606").Result()
	abc,_ := value.Result()
	abc2,_ := value2.Result()
	fmt.Println(abc)
	fmt.Println(abc2)


}