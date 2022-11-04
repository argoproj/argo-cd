package image

import (
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/tag"

	"github.com/distribution/distribution/v3/reference"
)

type ContainerImage struct {
	RegistryURL           string
	ImageName             string
	ImageTag              *tag.ImageTag
	ImageAlias            string
	HelmParamImageName    string
	HelmParamImageVersion string
	KustomizeImage        *ContainerImage
	original              string
}

type ContainerImageList []*ContainerImage

// NewFromIdentifier parses an image identifier and returns a populated ContainerImage
func NewFromIdentifier(identifier string) *ContainerImage {
	imgRef := identifier
	alias := ""
	if strings.Contains(identifier, "=") {
		n := strings.SplitN(identifier, "=", 2)
		imgRef = n[1]
		alias = n[0]
	}
	if parsed, err := reference.ParseNormalizedNamed(imgRef); err == nil {
		img := ContainerImage{}
		img.RegistryURL = reference.Domain(parsed)
		// remove default registry for backwards-compatibility
		if img.RegistryURL == "docker.io" && !strings.HasPrefix(imgRef, "docker.io") {
			img.RegistryURL = ""
		}
		img.ImageAlias = alias
		img.ImageName = reference.Path(parsed)
		// if library/ was added to the image name, remove it
		if !strings.HasPrefix(imgRef, "library/") {
			img.ImageName = strings.TrimPrefix(img.ImageName, "library/")
		}
		if digested, ok := parsed.(reference.Digested); ok {
			img.ImageTag = &tag.ImageTag{
				TagDigest: string(digested.Digest()),
			}
		} else if tagged, ok := parsed.(reference.Tagged); ok {
			img.ImageTag = &tag.ImageTag{
				TagName: tagged.Tag(),
			}
		}
		img.original = identifier
		return &img
	}

	// if distribution couldn't parse it, fall back to the legacy parsing logic
	img := ContainerImage{}
	img.RegistryURL = getRegistryFromIdentifier(identifier)
	img.ImageAlias, img.ImageName, img.ImageTag = getImageTagFromIdentifier(identifier)
	img.original = identifier
	return &img
}

// String returns the string representation of given ContainerImage
func (img *ContainerImage) String() string {
	str := ""
	if img.ImageAlias != "" {
		str += img.ImageAlias
		str += "="
	}
	str += img.GetFullNameWithTag()
	return str
}

func (img *ContainerImage) GetFullNameWithoutTag() string {
	str := ""
	if img.RegistryURL != "" {
		str += img.RegistryURL + "/"
	}
	str += img.ImageName
	return str
}

// GetFullNameWithTag returns the complete image slug, including the registry
// and any tag digest or tag name set for the image.
func (img *ContainerImage) GetFullNameWithTag() string {
	str := ""
	if img.RegistryURL != "" {
		str += img.RegistryURL + "/"
	}
	str += img.ImageName
	if img.ImageTag != nil {
		if img.ImageTag.TagDigest != "" {
			str += "@"
			str += img.ImageTag.TagDigest
		} else if img.ImageTag.TagName != "" {
			str += ":"
			str += img.ImageTag.TagName
		}
	}
	return str
}

// GetTagWithDigest returns tag name along with any tag digest set for the image
func (img *ContainerImage) GetTagWithDigest() string {
	str := ""
	if img.ImageTag != nil {
		if img.ImageTag.TagName != "" {
			str += img.ImageTag.TagName
		}
		if img.ImageTag.TagDigest != "" {
			if str == "" {
				str += "latest"
			}
			str += "@"
			str += img.ImageTag.TagDigest
		}
	}
	return str
}

func (img *ContainerImage) Original() string {
	return img.original
}

// IsUpdatable checks whether the given image can be updated with newTag while
// taking tagSpec into account. tagSpec must be given as a semver compatible
// version spec, i.e. ^1.0 or ~2.1
func (img *ContainerImage) IsUpdatable(newTag, tagSpec string) bool {
	return false
}

// WithTag returns a copy of img with new tag information set
func (img *ContainerImage) WithTag(newTag *tag.ImageTag) *ContainerImage {
	nimg := &ContainerImage{}
	nimg.RegistryURL = img.RegistryURL
	nimg.ImageName = img.ImageName
	nimg.ImageTag = newTag
	nimg.ImageAlias = img.ImageAlias
	nimg.HelmParamImageName = img.HelmParamImageName
	nimg.HelmParamImageVersion = img.HelmParamImageVersion
	return nimg
}

func (img *ContainerImage) DiffersFrom(other *ContainerImage, checkVersion bool) bool {
	return img.RegistryURL != other.RegistryURL || img.ImageName != other.ImageName || (checkVersion && img.ImageTag.TagName != other.ImageTag.TagName)
}

// ContainsImage checks whether img is contained in a list of images
func (list *ContainerImageList) ContainsImage(img *ContainerImage, checkVersion bool) *ContainerImage {
	// if there is a KustomizeImage override, check it for a match first
	if img.KustomizeImage != nil {
		if kustomizeMatch := list.ContainsImage(img.KustomizeImage, checkVersion); kustomizeMatch != nil {
			return kustomizeMatch
		}
	}
	for _, image := range *list {
		if img.ImageName == image.ImageName && image.RegistryURL == img.RegistryURL {
			if !checkVersion || image.ImageTag.TagName == img.ImageTag.TagName {
				return image
			}
		}
	}
	return nil
}

func (list *ContainerImageList) Originals() []string {
	results := make([]string, len(*list))
	for i, img := range *list {
		results[i] = img.Original()
	}
	return results
}

// String Returns the name of all images as a string, separated using comma
func (list *ContainerImageList) String() string {
	imgNameList := make([]string, 0)
	for _, image := range *list {
		imgNameList = append(imgNameList, image.String())
	}
	return strings.Join(imgNameList, ",")
}

// Gets the registry URL from an image identifier
func getRegistryFromIdentifier(identifier string) string {
	var imageString string
	comp := strings.Split(identifier, "=")
	if len(comp) > 1 {
		imageString = comp[1]
	} else {
		imageString = identifier
	}
	comp = strings.Split(imageString, "/")
	if len(comp) > 1 && strings.Contains(comp[0], ".") {
		return comp[0]
	} else {
		return ""
	}
}

// Gets the image name and tag from an image identifier
func getImageTagFromIdentifier(identifier string) (string, string, *tag.ImageTag) {
	var imageString string
	var sourceName string

	// The original name is prepended to the image name, separated by =
	comp := strings.SplitN(identifier, "=", 2)
	if len(comp) == 2 {
		sourceName = comp[0]
		imageString = comp[1]
	} else {
		imageString = identifier
	}

	// Strip any repository identifier from the string
	comp = strings.Split(imageString, "/")
	if len(comp) > 1 && strings.Contains(comp[0], ".") {
		imageString = strings.Join(comp[1:], "/")
	}

	// We can either have a tag name or a digest reference
	if strings.Contains(imageString, "@") {
		comp = strings.SplitN(imageString, "@", 2)
		return sourceName, comp[0], tag.NewImageTag("", time.Unix(0, 0), comp[1])
	} else {
		comp = strings.SplitN(imageString, ":", 2)
		if len(comp) != 2 {
			return sourceName, imageString, nil
		} else {
			tagName, tagDigest := getImageDigestFromTag(comp[1])
			return sourceName, comp[0], tag.NewImageTag(tagName, time.Unix(0, 0), tagDigest)
		}
	}
}

func getImageDigestFromTag(tagStr string) (string, string) {
	a := strings.Split(tagStr, "@")
	if len(a) != 2 {
		return tagStr, ""
	} else {
		return a[0], a[1]
	}
}

// LogContext returns a log context for the given image, with required fields
// set to the image's information.
func (img *ContainerImage) LogContext() *log.LogContext {
	logCtx := log.WithContext()
	logCtx.AddField("image_name", img.GetFullNameWithoutTag())
	logCtx.AddField("image_alias", img.ImageAlias)
	logCtx.AddField("registry_url", img.RegistryURL)
	if img.ImageTag != nil {
		logCtx.AddField("image_tag", img.ImageTag.TagName)
		logCtx.AddField("image_digest", img.ImageTag.TagDigest)
	}
	return logCtx
}
