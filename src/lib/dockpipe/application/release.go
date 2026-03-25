package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// env vars aligned with shipyard R2 publish and AWS CLI conventions.
const (
	envReleaseBucket  = "DOCKPIPE_RELEASE_BUCKET"
	envR2Bucket       = "R2_BUCKET"
	envR2Endpoint     = "R2_ENDPOINT_URL"
	envAWSEndpointS3  = "AWS_ENDPOINT_URL_S3"
	envCloudflareAcct = "CLOUDFLARE_ACCOUNT_ID"
	envR2AccountID    = "R2_ACCOUNT_ID"
	envAWSRegion      = "AWS_REGION"
)

func cmdRelease(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(releaseUsageText)
		return nil
	}
	switch args[0] {
	case "upload":
		return cmdReleaseUpload(args[1:])
	default:
		return fmt.Errorf("unknown release subcommand %q (try: dockpipe release --help)", args[0])
	}
}

func cmdReleaseUpload(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(releaseUploadUsageText)
		return nil
	}
	var (
		localPath   string
		bucket      string
		key         string
		endpoint    string
		region      string
		dryRun      bool
		contentType string
	)
	// First non-flag arg is the file path.
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--bucket" && i+1 < len(args):
			bucket = args[i+1]
			i++
		case a == "--key" && i+1 < len(args):
			key = args[i+1]
			i++
		case a == "--endpoint-url" && i+1 < len(args):
			endpoint = args[i+1]
			i++
		case a == "--region" && i+1 < len(args):
			region = args[i+1]
			i++
		case a == "--content-type" && i+1 < len(args):
			contentType = args[i+1]
			i++
		case a == "--dry-run":
			dryRun = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe release upload --help)", a)
		default:
			if localPath != "" {
				return fmt.Errorf("unexpected extra argument %q", a)
			}
			localPath = a
		}
	}
	if localPath == "" {
		return fmt.Errorf("missing local file path (try: dockpipe release upload --help)")
	}
	localPath = filepath.Clean(localPath)
	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("file %q: %w", localPath, err)
	}
	if bucket == "" {
		bucket = os.Getenv(envReleaseBucket)
		if bucket == "" {
			bucket = os.Getenv(envR2Bucket)
		}
	}
	if bucket == "" {
		return fmt.Errorf("set --bucket or %s or %s", envReleaseBucket, envR2Bucket)
	}
	if endpoint == "" {
		endpoint = resolveReleaseEndpoint()
	}
	if region == "" {
		region = os.Getenv(envAWSRegion)
	}
	if region == "" {
		if endpoint != "" {
			region = "auto"
		} else {
			region = "us-east-1"
		}
	}
	if key == "" {
		key = filepath.Base(localPath)
	}
	key = strings.TrimPrefix(key, "/")
	remote := fmt.Sprintf("s3://%s/%s", bucket, key)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dockpipe] dry-run: would upload %q to %q", localPath, remote)
		if endpoint != "" {
			fmt.Fprintf(os.Stderr, " endpoint=%q", endpoint)
		}
		fmt.Fprintln(os.Stderr)
		return nil
	}

	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return fmt.Errorf("aws CLI not found in PATH (install AWS CLI v2 for S3-compatible uploads)")
	}

	cmdArgs := []string{"s3", "cp", localPath, remote, "--region", region}
	if endpoint != "" {
		cmdArgs = append(cmdArgs, "--endpoint-url", endpoint)
	}
	if contentType != "" {
		cmdArgs = append(cmdArgs, "--content-type", contentType)
	}
	cmd := exec.Command(awsPath, cmdArgs...)
	cmd.Env = append(os.Environ(), "AWS_EC2_METADATA_DISABLED=true")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws s3 cp: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] uploaded %s\n", remote)
	return nil
}

func resolveReleaseEndpoint() string {
	if v := os.Getenv(envR2Endpoint); v != "" {
		return v
	}
	if v := os.Getenv(envAWSEndpointS3); v != "" {
		return v
	}
	acct := os.Getenv(envCloudflareAcct)
	if acct == "" {
		acct = os.Getenv(envR2AccountID)
	}
	if acct == "" {
		return ""
	}
	return "https://" + acct + ".r2.cloudflarestorage.com"
}

const releaseUsageText = `dockpipe release

Upload artifacts to a self-hosted S3-compatible bucket (e.g. Cloudflare R2).
Official DockPipe distribution does not require this; it is for dogfooding and
downstream registries. Credentials: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
(R2 API tokens or AWS keys). Same endpoint patterns as shipyard/workflows/r2-publish.

Usage:
  dockpipe release upload <local-file> [options]

`

const releaseUploadUsageText = `dockpipe release upload <local-file>

Uses aws s3 cp (AWS CLI v2). For R2, set endpoint via --endpoint-url or
R2_ENDPOINT_URL, or CLOUDFLARE_ACCOUNT_ID / R2_ACCOUNT_ID for the default
https://<id>.r2.cloudflarestorage.com URL.

Options:
  --bucket <name>       Bucket (or DOCKPIPE_RELEASE_BUCKET / R2_BUCKET)
  --key <object-key>    Object key (default: basename of local file)
  --endpoint-url <url>  S3 API URL (or R2_ENDPOINT_URL / AWS_ENDPOINT_URL_S3)
  --region <name>       AWS region (default: AWS_REGION, else auto if endpoint set, else us-east-1)
  --content-type <ct>   Optional Content-Type for the object
  --dry-run             Print destination only; do not invoke aws

Environment:
  AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY   S3-compatible credentials
  R2_BUCKET, R2_ENDPOINT_URL                 Common R2 names (see docs)

`
