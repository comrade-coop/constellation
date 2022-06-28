package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	azurecloud "github.com/edgelesssys/constellation/coordinator/cloudprovider/azure"
	gcpcloud "github.com/edgelesssys/constellation/coordinator/cloudprovider/gcp"
	"github.com/edgelesssys/constellation/coordinator/core"
	"github.com/edgelesssys/constellation/internal/attestation/azure"
	"github.com/edgelesssys/constellation/internal/attestation/gcp"
	"github.com/edgelesssys/constellation/internal/attestation/qemu"
	"github.com/edgelesssys/constellation/internal/attestation/vtpm"
	"github.com/edgelesssys/constellation/internal/constants"
	"github.com/edgelesssys/constellation/internal/logger"
	"github.com/edgelesssys/constellation/state/keyservice"
	"github.com/edgelesssys/constellation/state/mapper"
	"github.com/edgelesssys/constellation/state/setup"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	gcpStateDiskPath   = "/dev/disk/by-id/google-state-disk"
	azureStateDiskPath = "/dev/disk/azure/scsi1/lun0"
	qemuStateDiskPath  = "/dev/vda"
)

var csp = flag.String("csp", "", "Cloud Service Provider the image is running on")

func main() {
	flag.Parse()

	log := logger.New(logger.JSONLog, zapcore.InfoLevel)
	log.With(zap.String("version", constants.VersionInfo), zap.String("cloudProvider", *csp)).
		Infof("Starting disk-mapper")

	// set up metadata API and quote issuer for aTLS connections
	var err error
	var diskPathErr error
	var diskPath string
	var issuer core.QuoteIssuer
	var metadata core.ProviderMetadata
	switch strings.ToLower(*csp) {
	case "azure":
		diskPath, diskPathErr = filepath.EvalSymlinks(azureStateDiskPath)
		metadata, err = azurecloud.NewMetadata(context.Background())
		if err != nil {
			log.With(zap.Error).Fatalf("Failed to create Azure metadata API")
		}
		issuer = azure.NewIssuer()

	case "gcp":
		diskPath, diskPathErr = filepath.EvalSymlinks(gcpStateDiskPath)
		issuer = gcp.NewIssuer()
		gcpClient, err := gcpcloud.NewClient(context.Background())
		if err != nil {
			log.With(zap.Error).Fatalf("Failed to create GCP client")
		}
		metadata = gcpcloud.New(gcpClient)

	case "qemu":
		diskPath = qemuStateDiskPath
		issuer = qemu.NewIssuer()
		log.Warnf("cloud services are not supported on QEMU")
		metadata = &core.ProviderMetadataFake{}

	default:
		diskPathErr = fmt.Errorf("csp %q is not supported by Constellation", *csp)
	}
	if diskPathErr != nil {
		log.With(zap.Error(diskPathErr)).Fatalf("Unable to determine state disk path")
	}

	// initialize device mapper
	mapper, err := mapper.New(diskPath)
	if err != nil {
		log.With(zap.Error(err)).Fatalf("Failed to initialize device mapper")
	}
	defer mapper.Close()

	setupManger := setup.New(
		log.Named("setupManager"),
		*csp,
		afero.Afero{Fs: afero.NewOsFs()},
		keyservice.New(log.Named("keyService"), issuer, metadata, 20*time.Second), // try to request a key every 20 seconds
		mapper,
		setup.DiskMounter{},
		vtpm.OpenVTPM,
	)

	// prepare the state disk
	if mapper.IsLUKSDevice() {
		err = setupManger.PrepareExistingDisk()
	} else {
		err = setupManger.PrepareNewDisk()
	}
	if err != nil {
		log.With(zap.Error(err)).Fatalf("Failed to prepare state disk")
	}
}
