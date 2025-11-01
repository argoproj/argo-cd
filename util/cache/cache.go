package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/common"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	"github.com/argoproj/argo-cd/v3/util/env"
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
	// envRedisCredsFilePath is an env variable name which stores path to redis credentials file
	envRedisCredsFilePath = "REDIS_CREDS_FILE_PATH"
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

func getFlagVal[T any](cmd *cobra.Command, o Options, name string, getVal func(name string) (T, error)) func() T {
	return func() T {
		var res T
		var err error
		if o.FlagPrefix != "" && cmd.Flags().Changed(o.FlagPrefix+name) {
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

// loadRedisCreds loads Redis credentials either from file-based mounts or environment variables.
// If a mount path is provided, Redis credentials are expected to be read only from the mounted files.
// If no mount path is provided, the function falls back to reading credentials from environment variables
// to maintain backward compatibility.
func loadRedisCreds(mountPath string, opt Options) (username, password, sentinelUsername, sentinelPassword string, err error) {
	if mountPath != "" {
		log.Infof("Loading Redis credentials from mounted directory: %s", mountPath)
		if _, statErr := os.Stat(mountPath); statErr != nil {
			return "", "", "", "", fmt.Errorf("failed to access Redis credentials: mount path %q does not exist or is inaccessible: %w", mountPath, statErr)
		}
		password, err = readAuthDetailsFromFile(mountPath, "auth")
		if err != nil {
			return "", "", "", "", err
		}
		username, err = readAuthDetailsFromFile(mountPath, "auth_username")
		if err != nil {
			return "", "", "", "", err
		}
		sentinelUsername, err = readAuthDetailsFromFile(mountPath, "sentinel_username")
		if err != nil {
			return "", "", "", "", err
		}
		sentinelPassword, err = readAuthDetailsFromFile(mountPath, "sentinel_auth")
		if err != nil {
			return "", "", "", "", err
		}

		return username, password, sentinelUsername, sentinelPassword, nil
	}
	log.Info("Loading Redis credentials from environment variables")
	username = os.Getenv(envRedisUsername)
	password = os.Getenv(envRedisPassword)
	sentinelUsername = os.Getenv(envRedisSentinelUsername)
	sentinelPassword = os.Getenv(envRedisSentinelPassword)
	// If a flag prefix is set, prefer prefixed env vars to allow component-specific overrides (e.g., REPOSERVER_REDIS_PASSWORD).
	if opt.FlagPrefix != "" {
		pref := opt.getEnvPrefix()
		if val := os.Getenv(pref + envRedisUsername); val != "" {
			username = val
		}
		if val := os.Getenv(pref + envRedisPassword); val != "" {
			password = val
		}
		if val := os.Getenv(pref + envRedisSentinelUsername); val != "" {
			sentinelUsername = val
		}
		if val := os.Getenv(pref + envRedisSentinelPassword); val != "" {
			sentinelPassword = val
		}
	}
	return username, password, sentinelUsername, sentinelPassword, nil
}

func readAuthDetailsFromFile(mountPath, filename string) (string, error) {
	path := filepath.Join(mountPath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Expected when a particular credential is not used
			log.Infof("Redis credential file %s not found; using empty value for Redis credential %s", path, filename)
			return "", nil
		}
		return "", fmt.Errorf("failed to access Redis credential file %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...Options) func() (*Cache, error) {
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

	cmd.Flags().StringVar(&redisAddress, opt.FlagPrefix+"redis", env.StringFromEnv(opt.getEnvPrefix()+"REDIS_SERVER", ""), "Redis server hostname and port (e.g. argocd-redis:6379). ")
	redisAddressSrc := getFlagVal(cmd, opt, "redis", cmd.Flags().GetString)
	cmd.Flags().IntVar(&redisDB, opt.FlagPrefix+"redisdb", env.ParseNumFromEnv(opt.getEnvPrefix()+"REDISDB", 0, 0, math.MaxInt32), "Redis database.")
	redisDBSrc := getFlagVal(cmd, opt, "redisdb", cmd.Flags().GetInt)
	cmd.Flags().StringArrayVar(&sentinelAddresses, opt.FlagPrefix+"sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	sentinelAddressesSrc := getFlagVal(cmd, opt, "sentinel", cmd.Flags().GetStringArray)
	cmd.Flags().StringVar(&sentinelMaster, opt.FlagPrefix+"sentinelmaster", "master", "Redis sentinel master group name.")
	sentinelMasterSrc := getFlagVal(cmd, opt, "sentinelmaster", cmd.Flags().GetString)
	cmd.Flags().DurationVar(&defaultCacheExpiration, opt.FlagPrefix+"default-cache-expiration", env.ParseDurationFromEnv("ARGOCD_DEFAULT_CACHE_EXPIRATION", 24*time.Hour, 0, math.MaxInt64), "Cache expiration default")
	defaultCacheExpirationSrc := getFlagVal(cmd, opt, "default-cache-expiration", cmd.Flags().GetDuration)
	cmd.Flags().BoolVar(&redisUseTLS, opt.FlagPrefix+"redis-use-tls", false, "Use TLS when connecting to Redis. ")
	redisUseTLSSrc := getFlagVal(cmd, opt, "redis-use-tls", cmd.Flags().GetBool)
	cmd.Flags().StringVar(&redisClientCertificate, opt.FlagPrefix+"redis-client-certificate", "", "Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).")
	redisClientCertificateSrc := getFlagVal(cmd, opt, "redis-client-certificate", cmd.Flags().GetString)
	cmd.Flags().StringVar(&redisClientKey, opt.FlagPrefix+"redis-client-key", "", "Path to Redis client key (e.g. /etc/certs/redis/client.crt).")
	redisClientKeySrc := getFlagVal(cmd, opt, "redis-client-key", cmd.Flags().GetString)
	cmd.Flags().BoolVar(&insecureRedis, opt.FlagPrefix+"redis-insecure-skip-tls-verify", false, "Skip Redis server certificate validation.")
	insecureRedisSrc := getFlagVal(cmd, opt, "redis-insecure-skip-tls-verify", cmd.Flags().GetBool)
	cmd.Flags().StringVar(&redisCACertificate, opt.FlagPrefix+"redis-ca-certificate", "", "Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.")
	redisCACertificateSrc := getFlagVal(cmd, opt, "redis-ca-certificate", cmd.Flags().GetString)
	cmd.Flags().StringVar(&compressionStr, opt.FlagPrefix+CLIFlagRedisCompress, env.StringFromEnv(opt.getEnvPrefix()+"REDIS_COMPRESSION", string(RedisCompressionGZip)), "Enable compression for data sent to Redis with the required compression algorithm. (possible values: gzip, none)")
	compressionStrSrc := getFlagVal(cmd, opt, CLIFlagRedisCompress, cmd.Flags().GetString)
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

		var tlsConfig *tls.Config
		if redisUseTLS {
			tlsConfig = &tls.Config{}
			if redisClientCertificate != "" {
				clientCert, err := tls.LoadX509KeyPair(redisClientCertificate, redisClientKey)
				if err != nil {
					return nil, err
				}
				tlsConfig.Certificates = []tls.Certificate{clientCert}
			}
			switch {
			case insecureRedis:
				tlsConfig.InsecureSkipVerify = true
			case redisCACertificate != "":
				redisCA, err := certutil.ParseTLSCertificatesFromPath(redisCACertificate)
				if err != nil {
					return nil, err
				}
				tlsConfig.RootCAs = certutil.GetCertPoolFromPEMData(redisCA)
			default:
				var err error
				tlsConfig.RootCAs, err = x509.SystemCertPool()
				if err != nil {
					return nil, err
				}
			}
		}
		var password, username, sentinelUsername, sentinelPassword string
		credsFilePath := os.Getenv(envRedisCredsFilePath)
		if opt.FlagPrefix != "" {
			if val := os.Getenv(opt.getEnvPrefix() + envRedisCredsFilePath); val != "" {
				credsFilePath = val
			}
		}
		username, password, sentinelUsername, sentinelPassword, err := loadRedisCreds(credsFilePath, opt)
		if err != nil {
			return nil, err
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
func (c *Cache) SetItem(key string, item any, opts *CacheActionOpts) error {
	if item == nil {
		return errors.New("cannot set nil item in cache")
	}
	if opts == nil {
		opts = &CacheActionOpts{}
	}
	fullKey := c.generateFullKey(key)
	client := c.GetClient()
	if opts.Delete {
		return client.Delete(fullKey)
	}
	return client.Set(&Item{Key: fullKey, Object: item, CacheActionOpts: *opts})
}

func (c *Cache) GetItem(key string, item any) error {
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
