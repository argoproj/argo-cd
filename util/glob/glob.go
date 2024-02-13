package glob

import (
	"hash/fnv"
	"time"

	"github.com/gobwas/glob"
	"github.com/karlseguin/ccache/v3"
	log "github.com/sirupsen/logrus"
)

const (
	LRUCacheSize = 2048

	LRUShardsAmount  = 120
	LRUCacheLifetime = time.Hour * 24 * 7
)

var cache []*ccache.Cache[glob.Glob]

func init() {
	cache = make([]*ccache.Cache[glob.Glob], LRUShardsAmount)
	for i := 0; i < LRUShardsAmount; i++ {
		cache[i] = ccache.New(ccache.Configure[glob.Glob]().MaxSize(LRUCacheSize))
	}
}

func CachedCompile(pattern string, separators ...rune) (glob.Glob, error) {
	var shardId int
	hash := fnv.New32()

	if _, err := hash.Write([]byte(pattern)); err != nil {
		shardId = 0
	} else {
		shardId = int(hash.Sum32()) % LRUShardsAmount
	}

	if item := cache[shardId].Get(pattern); item != nil {
		return item.Value(), nil
	}

	if compiledGlob, err := glob.Compile(pattern, separators...); err != nil {
		return nil, err
	} else {
		cache[shardId].Set(pattern, compiledGlob, LRUCacheLifetime)
		// cache[shardId].Add(pattern, compiledGlob)
		return compiledGlob, nil
	}
}

func Match(pattern, text string, separators ...rune) bool {
	compiledGlob, err := CachedCompile(pattern, separators...)
	if err != nil {
		log.Warnf("failed to compile pattern %s due to error %v", pattern, err)
		return false
	}
	return compiledGlob.Match(text)
}
