package helm

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"golang.org/x/sync/singleflight"
	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

// Keep only successful body digests, never the bodies or their freshness and
// authentication metadata. Every read hashes its bytes, so a changed/corrupt
// body cannot reuse validation through matching timestamps or file sizes.
var validatedIndexBodies = newIndexValidationCache(256, parseIndexBody)

// Decode the pinned Helm index's complete schema without importing Helm's
// command dependencies. Semantic chart validation remains Helm's responsibility.
type indexValidationChart struct {
	*IndexValidationMetadata
	URLs                                         []string
	Created                                      time.Time
	Removed                                      bool
	Digest, Checksum, Engine, TillerVersion, URL string
}

// IndexValidationMetadata is exported so encoding/json can initialize the
// embedded pointer, matching the pinned Helm decoder's treatment of scalars.
type IndexValidationMetadata struct {
	Name, Home, Version, Description, Icon, APIVersion, Condition, Tags, AppVersion, KubeVersion, Type string
	Sources, Keywords                                                                                  []string
	Deprecated                                                                                         bool
	Annotations                                                                                        map[string]string
	Maintainers                                                                                        []*struct {
		Name, Email, URL string
	}
	Dependencies []*struct {
		Name, Version, Repository, Condition, Alias string
		Tags                                        []string
		Enabled                                     bool
		ImportValues                                []any `json:"import-values"`
	}
}

func validateIndexBody(data []byte) error {
	return validatedIndexBodies.validateBody(data)
}

type indexValidationCache struct {
	mu       sync.Mutex
	valid    *lru.Cache
	pending  singleflight.Group
	validate func([]byte) error
}

func newIndexValidationCache(limit int, validate func([]byte) error) *indexValidationCache {
	return &indexValidationCache{valid: lru.New(limit), validate: validate}
}

func (c *indexValidationCache) contains(digest [sha256.Size]byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.valid.Get(digest)
	return ok
}

func (c *indexValidationCache) validateBody(body []byte) error {
	digest := sha256.Sum256(body)
	return c.validateBodyDigest(body, digest)
}

// The caller computes this digest from the bytes, never from cache metadata.
func (c *indexValidationCache) validateBodyDigest(body []byte, digest [sha256.Size]byte) error {
	if c.contains(digest) {
		return nil
	}
	// Coalesce identical cold bodies without holding the cache mutex while
	// parsing. Other warm bodies remain usable during an expensive validation.
	_, err, _ := c.pending.Do(string(digest[:]), func() (any, error) {
		if c.contains(digest) {
			return nil, nil
		}
		if err := c.validate(body); err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.valid.Add(digest, struct{}{})
		c.mu.Unlock()
		return nil, nil
	})
	return err
}

// parseIndexBody validates that data is a complete Helm v1 repository index.
// Only bodies passing this check are ever eligible for conditional (304)
// revalidation or shared-cache publication.
func parseIndexBody(data []byte) error {
	// The schema decoder accepts only the first YAML document. Check the
	// complete stream too, so trailing corruption cannot acquire validators.
	// Successful validation is memoized by body digest, including warm reads.
	jsonBody := json.Valid(data)
	if !jsonBody {
		decoder := yamlv3.NewDecoder(bytes.NewReader(data))
		var document yamlv3.Node
		if err := decoder.Decode(&document); err != nil {
			return fmt.Errorf("helm index is not valid YAML: %w", err)
		}
		// Empty trailing documents are harmless: the pinned Helm decoder
		// also stops at the first document, empty or not. Only a trailing
		// document with actual content is corruption worth rejecting.
		for {
			err := decoder.Decode(&document)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("helm index is not valid YAML: %w", err)
			}
			// yamlv3 reports document nodes with an empty tag; look at the
			// child instead. An empty document decodes to an implicit null
			// scalar with no value. Check the node shape, not just the tag:
			// an explicit !!null tag can sit on a non-empty mapping.
			if len(document.Content) > 0 && !isImplicitEmptyNull(document.Content[0]) {
				return errors.New("helm index must contain exactly one YAML document")
			}
		}
	}
	var env struct {
		APIVersion  string                             `json:"apiVersion"`
		Generated   time.Time                          `json:"generated"`
		Entries     map[string][]*indexValidationChart `json:"entries"`
		ServerInfo  map[string]any                     `json:"serverInfo"`
		PublicKeys  []string                           `json:"publicKeys"`
		Annotations map[string]string                  `json:"annotations"`
	}
	// Match the pinned Helm loader: JSON permits unknown fields, YAML uses
	// strict schema decoding. Validation remains memoized for unchanged bytes.
	if jsonBody {
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("helm index is not valid JSON: %w", err)
		}
	} else if err := yaml.UnmarshalStrict(data, &env); err != nil {
		return fmt.Errorf("helm index is not valid YAML: %w", err)
	}
	// Only complete Helm v1 indexes can be published to shared storage.
	if env.APIVersion != "v1" {
		return fmt.Errorf("helm index has unsupported apiVersion %q", env.APIVersion)
	}
	return nil
}

// isImplicitEmptyNull reports whether node is what yamlv3 produces for an
// empty document: a null scalar with no content. Mappings, sequences and
// scalars with explicit content (including "~" or "null") are rejected.
func isImplicitEmptyNull(node *yamlv3.Node) bool {
	return node.Kind == yamlv3.ScalarNode && node.Tag == "!!null" && node.Value == ""
}
