/*
This file is part of Cloud Native PostgreSQL.

Copyright (C) 2019-2021 EnterpriseDB Corporation.
*/

package utils

import (
	"encoding/json"
	"fmt"
	"os"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	apiv1 "github.com/EnterpriseDB/cloud-native-postgresql/api/v1"

	. "github.com/onsi/gomega" // nolint
)

// ExecuteBackup performs a backup and check the backup status
func ExecuteBackup(namespace string, backupFile string, env *TestingEnvironment) {
	backupName, err := env.GetResourceNameFromYAML(backupFile)
	Expect(err).ToNot(HaveOccurred())

	_, _, err = Run(fmt.Sprintf(
		"kubectl apply -n %v -f %v",
		namespace, backupFile))
	Expect(err).ToNot(HaveOccurred())

	// After a while the Backup should be completed
	timeout := 180
	backupNamespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      backupName,
	}
	backup := &apiv1.Backup{}
	// Verifying backup status
	Eventually(func() (apiv1.BackupPhase, error) {
		err = env.Client.Get(env.Ctx, backupNamespacedName, backup)
		return backup.Status.Phase, err
	}, timeout).Should(BeEquivalentTo(apiv1.BackupPhaseCompleted))
	Eventually(func() (string, error) {
		err = env.Client.Get(env.Ctx, backupNamespacedName, backup)
		if err != nil {
			return "", err
		}
		backupStatus := backup.GetStatus()
		return backupStatus.BeginLSN, err
	}, timeout).ShouldNot(BeEmpty())

	backupStatus := backup.GetStatus()
	Expect(backupStatus.BeginWal).NotTo(BeEmpty())
	Expect(backupStatus.EndLSN).NotTo(BeEmpty())
	Expect(backupStatus.EndWal).NotTo(BeEmpty())
}

// CreateClusterFromBackupUsingPITR creates a cluster from backup, using the PITR
func CreateClusterFromBackupUsingPITR(
	namespace,
	clusterName,
	backupFilePath,
	targetTime string,
	env *TestingEnvironment) error {
	backupName, err := env.GetResourceNameFromYAML(backupFilePath)
	if err != nil {
		return err
	}
	storageClassName := os.Getenv("E2E_DEFAULT_STORAGE_CLASS")
	restoreCluster := &apiv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: apiv1.ClusterSpec{
			Instances: 3,

			StorageConfiguration: apiv1.StorageConfiguration{
				Size:         "1Gi",
				StorageClass: &storageClassName,
			},

			PostgresConfiguration: apiv1.PostgresConfiguration{
				Parameters: map[string]string{
					"log_checkpoints":             "on",
					"log_lock_waits":              "on",
					"log_min_duration_statement":  "1000",
					"log_statement":               "ddl",
					"log_temp_files":              "1024",
					"log_autovacuum_min_duration": "1s",
					"log_replication_commands":    "on",
				},
			},

			Bootstrap: &apiv1.BootstrapConfiguration{
				Recovery: &apiv1.BootstrapRecovery{
					Backup: &apiv1.BackupSource{
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: backupName,
						},
					},
					RecoveryTarget: &apiv1.RecoveryTarget{
						TargetTime: targetTime,
					},
				},
			},
		},
	}
	return env.Client.Create(env.Ctx, restoreCluster)
}

// CreateClusterFromExternalClusterBackupWithPITROnAzure creates a cluster on Azure, starting from an external cluster
// backup with PITR
func CreateClusterFromExternalClusterBackupWithPITROnAzure(
	namespace,
	externalClusterName,
	sourceClusterName,
	targetTime,
	storageCredentialsSecretName,
	azStorageAccount string,
	env *TestingEnvironment) error {
	storageClassName := os.Getenv("E2E_DEFAULT_STORAGE_CLASS")
	destinationPath := fmt.Sprintf("https://%v.blob.core.windows.net/%v/", azStorageAccount, sourceClusterName)

	restoreCluster := &apiv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      externalClusterName,
			Namespace: namespace,
		},
		Spec: apiv1.ClusterSpec{
			Instances: 3,

			StorageConfiguration: apiv1.StorageConfiguration{
				Size:         "1Gi",
				StorageClass: &storageClassName,
			},

			PostgresConfiguration: apiv1.PostgresConfiguration{
				Parameters: map[string]string{
					"log_checkpoints":             "on",
					"log_lock_waits":              "on",
					"log_min_duration_statement":  "1000",
					"log_statement":               "ddl",
					"log_temp_files":              "1024",
					"log_autovacuum_min_duration": "1s",
					"log_replication_commands":    "on",
				},
			},

			Bootstrap: &apiv1.BootstrapConfiguration{
				Recovery: &apiv1.BootstrapRecovery{
					Source: sourceClusterName,
					RecoveryTarget: &apiv1.RecoveryTarget{
						TargetTime: targetTime,
					},
				},
			},

			ExternalClusters: []apiv1.ExternalCluster{
				{
					Name: sourceClusterName,
					BarmanObjectStore: &apiv1.BarmanObjectStoreConfiguration{
						DestinationPath: destinationPath,
						AzureCredentials: &apiv1.AzureCredentials{
							StorageAccount: &apiv1.SecretKeySelector{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: storageCredentialsSecretName,
								},
								Key: "ID",
							},
							StorageKey: &apiv1.SecretKeySelector{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: storageCredentialsSecretName,
								},
								Key: "KEY",
							},
						},
					},
				},
			},
		},
	}

	return env.Client.Create(env.Ctx, restoreCluster)
}

// CreateClusterFromExternalClusterBackupWithPITROnMinio creates a cluster on Minio, starting from an external cluster
// backup with PITR
func CreateClusterFromExternalClusterBackupWithPITROnMinio(
	namespace,
	externalClusterName,
	sourceClusterName,
	targetTime string,
	env *TestingEnvironment) error {
	storageClassName := os.Getenv("E2E_DEFAULT_STORAGE_CLASS")

	restoreCluster := &apiv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      externalClusterName,
			Namespace: namespace,
		},
		Spec: apiv1.ClusterSpec{
			Instances: 3,

			StorageConfiguration: apiv1.StorageConfiguration{
				Size:         "1Gi",
				StorageClass: &storageClassName,
			},

			PostgresConfiguration: apiv1.PostgresConfiguration{
				Parameters: map[string]string{
					"log_checkpoints":             "on",
					"log_lock_waits":              "on",
					"log_min_duration_statement":  "1000",
					"log_statement":               "ddl",
					"log_temp_files":              "1024",
					"log_autovacuum_min_duration": "1s",
					"log_replication_commands":    "on",
				},
			},

			Bootstrap: &apiv1.BootstrapConfiguration{
				Recovery: &apiv1.BootstrapRecovery{
					Source: sourceClusterName,
					RecoveryTarget: &apiv1.RecoveryTarget{
						TargetTime: targetTime,
					},
				},
			},

			ExternalClusters: []apiv1.ExternalCluster{
				{
					Name: sourceClusterName,
					BarmanObjectStore: &apiv1.BarmanObjectStoreConfiguration{
						DestinationPath: "s3://cluster-backups/",
						EndpointURL:     "https://minio-service:9000",
						EndpointCA: &apiv1.SecretKeySelector{
							LocalObjectReference: apiv1.LocalObjectReference{
								Name: "minio-server-ca-secret",
							},
							Key: "ca.crt",
						},
						S3Credentials: &apiv1.S3Credentials{
							AccessKeyIDReference: &apiv1.SecretKeySelector{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "backup-storage-creds",
								},
								Key: "ID",
							},
							SecretAccessKeyReference: &apiv1.SecretKeySelector{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "backup-storage-creds",
								},
								Key: "KEY",
							},
						},
					},
				},
			},
		},
	}

	return env.Client.Create(env.Ctx, restoreCluster)
}

// CreateClusterFromExternalClusterBackupWithPITROnAzurite creates a cluster with Azurite, starting from an external
// cluster backup with PITR
func CreateClusterFromExternalClusterBackupWithPITROnAzurite(
	namespace,
	externalClusterName,
	sourceClusterName,
	targetTime string,
	env *TestingEnvironment) error {
	storageClassName := os.Getenv("E2E_DEFAULT_STORAGE_CLASS")
	DestinationPath := fmt.Sprintf("https://azurite:10000/storageaccountname/%v", sourceClusterName)

	restoreCluster := &apiv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      externalClusterName,
			Namespace: namespace,
		},
		Spec: apiv1.ClusterSpec{
			Instances: 3,

			StorageConfiguration: apiv1.StorageConfiguration{
				Size:         "1Gi",
				StorageClass: &storageClassName,
			},

			PostgresConfiguration: apiv1.PostgresConfiguration{
				Parameters: map[string]string{
					"log_checkpoints":             "on",
					"log_lock_waits":              "on",
					"log_min_duration_statement":  "1000",
					"log_statement":               "ddl",
					"log_temp_files":              "1024",
					"log_autovacuum_min_duration": "1s",
					"log_replication_commands":    "on",
				},
			},

			Bootstrap: &apiv1.BootstrapConfiguration{
				Recovery: &apiv1.BootstrapRecovery{
					Source: sourceClusterName,
					RecoveryTarget: &apiv1.RecoveryTarget{
						TargetTime: targetTime,
					},
				},
			},

			ExternalClusters: []apiv1.ExternalCluster{
				{
					Name: sourceClusterName,
					BarmanObjectStore: &apiv1.BarmanObjectStoreConfiguration{
						DestinationPath: DestinationPath,
						EndpointCA: &apiv1.SecretKeySelector{
							LocalObjectReference: apiv1.LocalObjectReference{
								Name: "azurite-ca-secret",
							},
							Key: "ca.crt",
						},
						AzureCredentials: &apiv1.AzureCredentials{
							ConnectionString: &apiv1.SecretKeySelector{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "azurite",
								},
								Key: "AZURE_CONNECTION_STRING",
							},
						},
					},
				},
			},
		},
	}

	return env.Client.Create(env.Ctx, restoreCluster)
}

// ComposeAzBlobListAzuriteCmd builds the Azure storage blob list command for Azurite
func ComposeAzBlobListAzuriteCmd(clusterName string, path string) string {
	return fmt.Sprintf("az storage blob list --container-name %v --query \"[?contains(@.name, \\`%v\\`)].name\" "+
		"--connection-string $AZURE_CONNECTION_STRING",
		clusterName, path)
}

// ComposeAzBlobListCmd builds the Azure storage blob list command
func ComposeAzBlobListCmd(azStorageAccount, azStorageKey, clusterName string, path string) string {
	return fmt.Sprintf("az storage blob list --account-name %v  "+
		"--account-key %v  "+
		"--container-name %v --query \"[?contains(@.name, \\`%v\\`)].name\"",
		azStorageAccount, azStorageKey, clusterName, path)
}

// CountFilesOnAzureBlobStorage counts files on Azure Blob storage
func CountFilesOnAzureBlobStorage(
	azStorageAccount string,
	azStorageKey string,
	clusterName string,
	path string) (int, error) {
	azBlobListCmd := ComposeAzBlobListCmd(azStorageAccount, azStorageKey, clusterName, path)
	out, _, err := RunUnchecked(azBlobListCmd)
	if err != nil {
		return -1, err
	}
	var arr []string
	err = json.Unmarshal([]byte(out), &arr)
	return len(arr), err
}

// CountFilesOnAzuriteBlobStorage counts files on Azure Blob storage. using Azurite
func CountFilesOnAzuriteBlobStorage(
	namespace,
	clusterName string,
	path string) (int, error) {
	azBlobListCmd := ComposeAzBlobListAzuriteCmd(clusterName, path)
	out, _, err := RunUnchecked(fmt.Sprintf("kubectl exec -n %v az-cli "+
		"-- /bin/bash -c '%v'", namespace, azBlobListCmd))
	if err != nil {
		return -1, err
	}
	var arr []string
	err = json.Unmarshal([]byte(out), &arr)
	return len(arr), err
}
