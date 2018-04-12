package config

import (
	"os"
	"time"
)

// All configs that depend on environments
func ClusterBaseImage() string {
	// TODO: These are the publicly available Postverta images,
	// which is built with the postverta/base_image repository on
	// GitHub. You may wany to build your own image.
	if os.Getenv("PRODUCTION") != "" {
		return "postverta/base:latest"
	} else {
		return "postverta/base_dev:latest"
	}
}

func ClusterEndpoints() []string {
	// TODO: a list of endpoints with pv_agent running
	if os.Getenv("PRODUCTION") != "" {
		return []string{
			"compute-prod-0:8080",
			"compute-prod-1:8080",
			"compute-prod-2:8080",
		}
	} else {
		return []string{
			"compute-dev:8080",
		}
	}
}

func ClusterContextExpirationTime() time.Duration {
	// TODO: how long a container is shutdown after inactivity
	if os.Getenv("PRODUCTION") != "" {
		return time.Minute * 10
	} else {
		return time.Second * 30
	}
}

func InternalApiEndPoint() string {
	// TODO: an HTTP endpoint of the pv_backend internal API
	// service.
	if os.Getenv("PRODUCTION") != "" {
		return "http://api:9091"
	} else {
		return "http://workstation:9091"
	}
}

func Auth0Secret() []byte {
	// TODO: the secret used to verify Auth0 JWT signature for
	// incoming HTTP requests.
	if os.Getenv("PRODUCTION") != "" {
		return []byte("FILL_ME_IN")
	} else {
		return []byte("FILL_ME_IN")
	}
}

func LogDirectory() string {
	// TODO: the directory to store app log outputs. Better use a
	// mounted remote file system as there can be a lot of log
	// files.
	if os.Getenv("PRODUCTION") != "" {
		return "/mnt/log/prod"
	} else {
		return "/mnt/log/dev"
	}
}

func LogIdleDuration() time.Duration {
	// TODO: parameters to decide when to close a log file handler
	// after inactivity. Not very critical.
	if os.Getenv("PRODUCTION") != "" {
		return 60 * time.Second
	} else {
		return 10 * time.Second
	}
}

func SegmentWriteKey() string {
	// TODO: key for the Segment.io event tracking service. We use
	// Segment on the backend only to track the requests to access
	// the app themselves, not to the Postverta APIs.
	if os.Getenv("PRODUCTION") != "" {
		return "FILL_ME_IN"
	} else {
		return "FILL_ME_IN"
	}
}

func AzureAccountName() string {
	// TODO: Azure storage account name for storing the file system
	// images.
	if os.Getenv("PRODUCTION") != "" {
		return "fill_me_in"
	} else {
		return "fill_me_in"
	}
}

func AzureAccountKey() string {
	// TODO: Azure storage account key
	if os.Getenv("PRODUCTION") != "" {
		return "FILL_ME_IN"
	} else {
		return "FILL_ME_IN"
	}
}

func CloudinaryUploadUrl() string {
	// TODO: Cloudinary upload URL
	return "https://api.cloudinary.com/v1_1/postverta/image/upload"
}

func CloudinaryDownloadUrl() string {
	// TODO: Cloudinary download URL including the iconify transformer
	return "https://res.cloudinary.com/postverta/image/upload/t_iconify"
}

func CloudinaryUploadPreset() string {
	// TODO: Cloudinary upload preset (secret)
	return "fill_me_in"
}

func WorktreeAutosaveInterval() uint32 {
	// TODO: The time interval (in seconds) of taking periodic snapshots of
	// an in-memory workspace. The smaller the number is, the less likely
	// for any data loss if the container crashes, but there will be higher
	// performance impact to I/O.
	if os.Getenv("PRODUCTION") != "" {
		return 30
	} else {
		return 5
	}
}
