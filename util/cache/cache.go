package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	flag "github.com/spf13/pflag"
	"math"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
	certutil "github.com/argoproj/argo-cd/v2/util/cert"
	"github.com/argoproj/argo-cd/v2/util/env"
)

const (
	// envRedisPassword is an env variable name which stores redis password
	envRedisPassword = "REDIS_PASSWORD"
	// envRedisUsername is an env variable name which stores redis username (for acl setup)
	envRedisUsername = "REDIS_USERNAME"
	// envRedisRetryCount is an env variable name which stores redis retry count
	envRedisRetryCount = "REDIS_RETRY_COUNT"
	// defaultRedisRetryCount holds default number of retries
	defaultRedisRetryCount = 3
	// envRedisSentinelPassword is an env variable name which stores redis sentinel password
	envRedisSentinelPassword = "REDIS_SENTINEL_PASSWORD"
	// envRedisSentinelUsername is an env variable name which stores redis sentinel username
	envRedisSentinelUsername = "REDIS_SENTINEL_USERNAME"
)

const (
	// CLIFlagRedisCompress is a cli flag name to define the redis compression setting for data sent to redis
	CLIFlagRedisCompress = "redis-compress"
)

func NewCache(client CacheClient) *Cache {
	return &Cache{client}
}

func buildRedisClient(redisAddress, password, username string, redisDB, maxRetries int, tlsConfig *tls.Config) *redis.Client {
	opts := &redis.Options{
		Addr:       redisAddress,
		Password:   password,
		DB:         redisDB,
		MaxRetries: maxRetries,
		TLSConfig:  tlsConfig,
		Username:   username,
	}

	client := redis.NewClient(opts)

	client.AddHook(redis.Hook(NewArgoRedisHook(func() {
		*client = *buildRedisClient(redisAddress, password, username, redisDB, maxRetries, tlsConfig)
	})))

	return client
}

func buildFailoverRedisClient(sentinelMaster, sentinelUsername, sentinelPassword, password, username string, redisDB, maxRetries int, tlsConfig *tls.Config, sentinelAddresses []string) *redis.Client {
	opts := &redis.FailoverOptions{
		MasterName:       sentinelMaster,
		SentinelAddrs:    sentinelAddresses,
		DB:               redisDB,
		Password:         password,
		MaxRetries:       maxRetries,
		TLSConfig:        tlsConfig,
		Username:         username,
		SentinelUsername: sentinelUsername,
		SentinelPassword: sentinelPassword,
	}

	client := redis.NewFailoverClient(opts)

	client.AddHook(redis.Hook(NewArgoRedisHook(func() {
		*client = *buildFailoverRedisClient(sentinelMaster, sentinelUsername, sentinelPassword, password, username, redisDB, maxRetries, tlsConfig, sentinelAddresses)
	})))

	return client
}

type Options struct {
	FlagPrefix      string
	OnClientCreated func(client *redis.Client)
}

func (o *Options) callOnClientCreated(client *redis.Client) {
	if o.OnClientCreated != nil {
		o.OnClientCreated(client)
	}
}

func (o *Options) getEnvPrefix() string {
	return strings.ReplaceAll(strings.ToUpper(o.FlagPrefix), "-", "_")
}

func mergeOptions(opts ...Options) Options {
	var result Options
	for _, o := range opts {
		if o.FlagPrefix != "" {
			result.FlagPrefix = o.FlagPrefix
		}
		if o.OnClientCreated != nil {
			result.OnClientCreated = o.OnClientCreated
		}
	}
	return result
}

func getFlagVal[T any](flags *flag.FlagSet, o Options, name string, getVal func(name string) (T, error)) func() T {
	return func() T {
		var res T
		var err error
		if o.FlagPrefix != "" && flags.Changed(o.FlagPrefix+name) {
			res, err = getVal(o.FlagPrefix + name)
		} else {
			res, err = getVal(name)
		}
		if err != nil {
			panic(err)
		}
		return res
	}
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(flags *flag.FlagSet, opts ...Options) func() (*Cache, error) {
	redisAddress := ""
	sentinelAddresses := make([]string, 0)
	sentinelMaster := ""
	redisDB := 0
	redisCACertificate := ""
	redisClientCertificate := ""
	redisClientKey := ""
	redisUseTLS := false
	insecureRedis := false
	compressionStr := ""
	opt := mergeOptions(opts...)
	var defaultCacheExpiration time.Duration

	flags.StringVar(&redisAddress, opt.FlagPrefix+"redis", env.StringFromEnv(opt.getEnvPrefix()+"REDIS_SERVER", ""), "Redis server hostname and port (e.g. argocd-redis:6379). ")
	redisAddressSrc := getFlagVal(flags, opt, "redis", flags.GetString)
	flags.IntVar(&redisDB, opt.FlagPrefix+"redisdb", env.ParseNumFromEnv(opt.getEnvPrefix()+"REDISDB", 0, 0, math.MaxInt32), "Redis database.")
	redisDBSrc := getFlagVal(flags, opt, "redisdb", flags.GetInt)
	flags.StringArrayVar(&sentinelAddresses, opt.FlagPrefix+"sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	sentinelAddressesSrc := getFlagVal(flags, opt, "sentinel", flags.GetStringArray)
	flags.StringVar(&sentinelMaster, opt.FlagPrefix+"sentinelmaster", "master", "Redis sentinel master group name.")
	sentinelMasterSrc := getFlagVal(flags, opt, "sentinelmaster", flags.GetString)
	flags.DurationVar(&defaultCacheExpiration, opt.FlagPrefix+"default-cache-expiration", env.ParseDurationFromEnv("ARGOCD_DEFAULT_CACHE_EXPIRATION", 24*time.Hour, 0, math.MaxInt64), "Cache expiration default")
	defaultCacheExpirationSrc := getFlagVal(flags, opt, "default-cache-expiration", flags.GetDuration)
	flags.BoolVar(&redisUseTLS, opt.FlagPrefix+"redis-use-tls", false, "Use TLS when connecting to Redis. ")
	redisUseTLSSrc := getFlagVal(flags, opt, "redis-use-tls", flags.GetBool)
	flags.StringVar(&redisClientCertificate, opt.FlagPrefix+"redis-client-certificate", "", "Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).")
	redisClientCertificateSrc := getFlagVal(flags, opt, "redis-client-certificate", flags.GetString)
	flags.StringVar(&redisClientKey, opt.FlagPrefix+"redis-client-key", "", "Path to Redis client key (e.g. /etc/certs/redis/client.crt).")
	redisClientKeySrc := getFlagVal(flags, opt, "redis-client-key", flags.GetString)
	flags.BoolVar(&insecureRedis, opt.FlagPrefix+"redis-insecure-skip-tls-verify", false, "Skip Redis server certificate validation.")
	insecureRedisSrc := getFlagVal(flags, opt, "redis-insecure-skip-tls-verify", flags.GetBool)
	flags.StringVar(&redisCACertificate, opt.FlagPrefix+"redis-ca-certificate", "", "Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.")
	redisCACertificateSrc := getFlagVal(flags, opt, "redis-ca-certificate", flags.GetString)
	flags.StringVar(&compressionStr, opt.FlagPrefix+CLIFlagRedisCompress, env.StringFromEnv(opt.getEnvPrefix()+"REDIS_COMPRESSION", string(RedisCompressionGZip)), "Enable compression for data sent to Redis with the required compression algorithm. (possible values: gzip, none)")
	compressionStrSrc := getFlagVal(flags, opt, CLIFlagRedisCompress, flags.GetString)
	return func() (*Cache, error) {
		redisAddress := redisAddressSrc()
		redisDB := redisDBSrc()
		sentinelAddresses := sentinelAddressesSrc()
		sentinelMaster := sentinelMasterSrc()
		defaultCacheExpiration := defaultCacheExpirationSrc()
		redisUseTLS := redisUseTLSSrc()
		redisClientCertificate := redisClientCertificateSrc()
		redisClientKey := redisClientKeySrc()
		insecureRedis := insecureRedisSrc()
		redisCACertificate := redisCACertificateSrc()
		compressionStr := compressionStrSrc()

		var tlsConfig *tls.Config = nil
		if redisUseTLS {
			tlsConfig = &tls.Config{}
			if redisClientCertificate != "" {
				clientCert, err := tls.LoadX509KeyPair(redisClientCertificate, redisClientKey)
				if err != nil {
					return nil, err
				}
				tlsConfig.Certificates = []tls.Certificate{clientCert}
			}
			if insecureRedis {
				tlsConfig.InsecureSkipVerify = true
			} else if redisCACertificate != "" {
				redisCA, err := certutil.ParseTLSCertificatesFromPath(redisCACertificate)
				if err != nil {
					return nil, err
				}
				tlsConfig.RootCAs = certutil.GetCertPoolFromPEMData(redisCA)
			} else {
				var err error
				tlsConfig.RootCAs, err = x509.SystemCertPool()
				if err != nil {
					return nil, err
				}
			}
		}
		password := os.Getenv(envRedisPassword)
		username := os.Getenv(envRedisUsername)
		sentinelUsername := os.Getenv(envRedisSentinelUsername)
		sentinelPassword := os.Getenv(envRedisSentinelPassword)
		if opt.FlagPrefix != "" {
			if val := os.Getenv(opt.getEnvPrefix() + envRedisUsername); val != "" {
				username = val
			}
			if val := os.Getenv(opt.getEnvPrefix() + envRedisPassword); val != "" {
				password = val
			}
			if val := os.Getenv(opt.getEnvPrefix() + envRedisSentinelUsername); val != "" {
				sentinelUsername = val
			}
			if val := os.Getenv(opt.getEnvPrefix() + envRedisSentinelPassword); val != "" {
				sentinelPassword = val
			}
		}

		maxRetries := env.ParseNumFromEnv(envRedisRetryCount, defaultRedisRetryCount, 0, math.MaxInt32)
		compression, err := CompressionTypeFromString(compressionStr)
		if err != nil {
			return nil, err
		}
		if len(sentinelAddresses) > 0 {
			client := buildFailoverRedisClient(sentinelMaster, sentinelUsername, sentinelPassword, password, username, redisDB, maxRetries, tlsConfig, sentinelAddresses)
			opt.callOnClientCreated(client)
			return NewCache(NewRedisCache(client, defaultCacheExpiration, compression)), nil
		}
		if redisAddress == "" {
			redisAddress = common.DefaultRedisAddr
		}

		client := buildRedisClient(redisAddress, password, username, redisDB, maxRetries, tlsConfig)
		opt.callOnClientCreated(client)
		return NewCache(NewRedisCache(client, defaultCacheExpiration, compression)), nil
	}
}

// Cache provides strongly types methods to store and retrieve values from shared cache
type Cache struct {
	client CacheClient
}

func (c *Cache) GetClient() CacheClient {
	return c.client
}

func (c *Cache) SetClient(client CacheClient) {
	c.client = client
}

func (c *Cache) RenameItem(oldKey string, newKey string, expiration time.Duration) error {
	return c.client.Rename(fmt.Sprintf("%s|%s", oldKey, common.CacheVersion), fmt.Sprintf("%s|%s", newKey, common.CacheVersion), expiration)
}

func (c *Cache) generateFullKey(key string) string {
	if key == "" {
		log.Debug("Cache key is empty, this will result in key collisions if there is more than one empty key")
	}
	return fmt.Sprintf("%s|%s", key, common.CacheVersion)
}

// Sets or deletes an item in cache
func (c *Cache) SetItem(key string, item interface{}, opts *CacheActionOpts) error {
	if item == nil {
		return fmt.Errorf("cannot set nil item in cache")
	}
	if opts == nil {
		opts = &CacheActionOpts{}
	}
	fullKey := c.generateFullKey(key)
	client := c.GetClient()
	if opts.Delete {
		return client.Delete(fullKey)
	} else {
		return client.Set(&Item{Key: fullKey, Object: item, CacheActionOpts: *opts})
	}
}

func (c *Cache) GetItem(key string, item interface{}) error {
	key = c.generateFullKey(key)
	if item == nil {
		return fmt.Errorf("cannot get item into a nil for key %s", key)
	}
	client := c.GetClient()
	return client.Get(key, item)
}

func (c *Cache) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return c.client.OnUpdated(ctx, c.generateFullKey(key), callback)
}

func (c *Cache) NotifyUpdated(key string) error {
	return c.client.NotifyUpdated(c.generateFullKey(key))
}
