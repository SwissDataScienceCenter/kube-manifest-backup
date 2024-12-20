// Main is the only package for kube-manifest-backup
package main

import (
	"bytes"
	"context"
	encodingjson "encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/go-co-op/gocron"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/sync"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/s3"
)

type kmbmetrics struct {
	backupCount   *prometheus.GaugeVec
	yamlFileCount *prometheus.GaugeVec
}

type resourceconfig struct {
	Resources []struct {
		Namespaces []string `yaml:"namespaces"`
		Group      string   `yaml:"group"`
		Version    string   `yaml:"version"`
		Resource   string   `yaml:"resource"`
		Secret     bool     `yaml:"secret"`
	} `yaml:"resources"`
}

type resourceinfo struct {
	Namespace *string
	Group     string
	Version   string
	Resource  string
	Secret    bool
}

type s3Config struct {
	ConfigFile string
	BucketName string
	BackupDir  string
}

func main() {
	// Parse command line flags, and environment variables
	usePrivateGPGKey := flag.Bool("use-private-gpg-key", getEnv("KMB_USE_PRIVATE_GPG_KEY", "false") == "true", "use a private GPG key to encrypt backups")
	privateKeySecretName := flag.String("private-key-secret-name", getEnv("KMB_PRIVATE_KEY_SECRET_NAME", "sops-gpg"), "name of the secret containing the private key")
	privateKeySecretNamespace := flag.String("private-key-secret-namespace", getEnv("KMB_PRIVATE_KEY_SECRET_NAMESPACE", "flux-system"), "namespace of the secret containing the private key")
	privateKeySecretKey := flag.String("private-key-secret-key", getEnv("KMB_PRIVATE_KEY_SECRET_KEY", "sops.asc"), "key in the secret containing the private key")
	backupSchedule := flag.String("backup-schedule", getEnv("KMB_BACKUP_SCHEDULE", "1/1 * * * *"), "cron schedule for backups")
	localBackupDir := flag.String("local-backup-dir", getEnv("KMB_LOCAL_BACKUP_DIR", "backups"), "local directory to store backups")
	runOnce := flag.Bool("run-once", getEnv("KMB_RUN_ONCE", "false") == "true", "run a single backup and exit")
	inCluster := flag.Bool("in-cluster", getEnv("KMB_IN_CLUSTER", "false") == "true", "use in-cluster config")
	backupResourcesYamlFile := flag.String("backup-resources-yaml-file", getEnv("KMB_BACKUP_RESOURCES_YAML_FILE", "resources.yaml"), "YAML file containing resources to backup")
	s3ConfigFile := flag.String("s3-config-file", getEnv("KMB_S3_CONFIG_FILE", "s3-config.json"), "S3 configuration file")
	s3BucketName := flag.String("s3-bucket-name", getEnv("KMB_S3_BUCKET_NAME", "kube-manifest-backup"), "S3 bucket name")
	s3BackupDir := flag.String("s3-backup-dir", getEnv("KMB_S3_BACKUP_DIR", "target-directory"), "S3 backup directory")

	var config *rest.Config
	var err error

	if *inCluster {
		// Creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		flag.Parse()
	} else {
		// Creates the out-of-cluster config
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}

		// Check if the KUBECONFIG environment variable is set
		if envKubeConfig, exists := os.LookupEnv("KUBECONFIG"); exists {
			*kubeconfig = envKubeConfig
		}

		flag.Parse()

		// Check if the KUBECONFIG environment variable is set
		if envKubeConfig, exists := os.LookupEnv("KUBECONFIG"); exists && *kubeconfig == "" {
			*kubeconfig = envKubeConfig
		}

		// Check if the kubeconfig file exists
		if _, err := os.Stat(*kubeconfig); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "kubeconfig file does not exist: %s\n", *kubeconfig)
			os.Exit(1)
		}

		// Use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// Initialise dynamicClient
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	privateKey := ""
	if *usePrivateGPGKey {
		// Create the clientSet
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		// Get private key secret key from Kubernetes secret in flux-system namespace
		secret, err := clientSet.CoreV1().Secrets(*privateKeySecretNamespace).Get(context.TODO(), *privateKeySecretName, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Failed to get %v secret: %v", privateKeySecretKey, err)
		}

		// Get private key secret key value
		if encodedValue, exists := secret.Data[*privateKeySecretKey]; exists {
			privateKey = string(encodedValue)
		}
	}

	var s3configuration s3Config
	s3configuration.ConfigFile = *s3ConfigFile
	s3configuration.BucketName = *s3BucketName
	s3configuration.BackupDir = *s3BackupDir

	// If singleBackup flag is set, run a single backup and exit
	if *runOnce {
		backupResources(dynamicClient, *localBackupDir, nil, privateKey, *backupResourcesYamlFile, s3configuration)
		os.Exit(0)
	}

	// Else, initialise Prometheus metrics and schedule backups
	kmbMetrics := initialiseMetrics()

	scheduleBackups(*backupSchedule, dynamicClient, *localBackupDir, &kmbMetrics, privateKey, *backupResourcesYamlFile, s3configuration)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}

func backupResources(dynamicClient dynamic.Interface, localBackupDir string, kmbMetrics *kmbmetrics, privateKey string, backupResourcesYamlFile string, s3Configuration s3Config) {
	resources := processResources(dynamicClient, localBackupDir, kmbMetrics, privateKey, backupResourcesYamlFile)
	cleanupOldBackupDirectories(localBackupDir, resources)

	ctx := context.Background()
	configfile.Install()

	s3config := loadS3Config(s3Configuration.ConfigFile)
	fdest, fsrc := setupFilesystems(ctx, localBackupDir, s3Configuration, s3config)

	performSync(ctx, fdest, fsrc)
}

func processResources(dynamicClient dynamic.Interface, localBackupDir string, kmbMetrics *kmbmetrics, privateKey string, backupResourcesYamlFile string) []resourceinfo {
	resources := readResourcesFromYAML(backupResourcesYamlFile)

	for _, resource := range resources {
		gvr := schema.GroupVersionResource{
			Group:    resource.Group,
			Version:  resource.Version,
			Resource: resource.Resource,
		}
		getResourceYAML(dynamicClient, resource.Namespace, resource.Resource, gvr, localBackupDir, kmbMetrics, resource.Secret, privateKey)
	}

	return resources
}

func loadS3Config(configFile string) map[string]string {
	file, err := os.Open(configFile)
	if err != nil {
		log.Println("Error opening S3 config file:", err)
		return nil
	}
	defer file.Close()

	byteValue, _ := io.ReadAll(file)

	var s3config map[string]string
	encodingjson.Unmarshal(byteValue, &s3config)

	return s3config
}

func setupFilesystems(ctx context.Context, localBackupDir string, s3Configuration s3Config, s3config map[string]string) (fs.Fs, fs.Fs) {
	m := configmap.Simple{}
	for key, value := range s3config {
		m.Set(key, value)
	}

	l := configmap.Simple{}
	l.Set("type", "local")

	fdest, err := s3.NewFs(ctx, "myS3", s3Configuration.BucketName+"/"+s3Configuration.BackupDir+"/", m)
	if err != nil {
		log.Fatalf("Failed to create filesystem for destination using alias: %v", err)
	}

	log.Println("Creating local filesystem for source")
	fsrc, err := local.NewFs(ctx, "myLocal", localBackupDir+"/", l)
	if err != nil {
		log.Fatalf("Failed to create filesystem for source: %v", err)
	}

	return fdest, fsrc
}

func performSync(ctx context.Context, fdest fs.Fs, fsrc fs.Fs) {
	err := sync.Sync(ctx, fdest, fsrc, true)
	if err != nil {
		log.Fatalf("Failed to sync: %v", err)
	}
}

func cleanupOldBackupDirectories(localBackupDir string, resources []resourceinfo) {
	entries, err := os.ReadDir(localBackupDir)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			resourceName := entry.Name()
			found := false
			for _, resource := range resources {
				if resource.Resource == resourceName {
					found = true
					break
				}
			}
			if !found {
				err := os.RemoveAll(fmt.Sprintf("%s/%s", localBackupDir, resourceName))
				if err != nil {
					log.Fatalf("Failed to remove directory: %v", err)
				}
			}
		}
	}
}

func readResourcesFromYAML(filename string) []resourceinfo {
	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read YAML file: %v", err)
	}

	var resourceconfig resourceconfig
	err = yaml.Unmarshal(yamlFile, &resourceconfig)
	if err != nil {
		log.Fatalf("Failed to unmarshal YAML content: %v", err)
	}

	var resources []resourceinfo
	for _, res := range resourceconfig.Resources {
		for _, namespace := range res.Namespaces {
			resources = append(resources, resourceinfo{
				Namespace: getNamespacePointer(namespace),
				Group:     res.Group,
				Version:   res.Version,
				Resource:  res.Resource,
				Secret:    res.Secret,
			})
		}
	}
	return resources
}

func getNamespacePointer(namespace string) *string {
	if namespace == "" {
		return nil
	}
	return &namespace
}

func getResourceYAML(dynamicClient dynamic.Interface, namespace *string, resourceType string, gvr schema.GroupVersionResource, localBackupDir string, kmbMetrics *kmbmetrics, isSecret bool, privateKey string) {
	var resources *unstructured.UnstructuredList
	var err error

	if namespace == nil {
		resources, err = dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	} else {
		resources, err = dynamicClient.Resource(gvr).Namespace(*namespace).List(context.TODO(), metav1.ListOptions{})
	}
	if err != nil {
		log.Printf("Error getting %s: %v\n", resourceType, err)
		os.Exit(1)
	}

	err = os.MkdirAll(fmt.Sprintf("%s/%s", localBackupDir, resourceType), 0755)
	if err != nil {
		log.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	writtenFiles := make(map[string]bool)
	yamlFileCount := 0

	items := reflect.ValueOf(resources).Elem().FieldByName("Items")
	for i := 0; i < items.Len(); i++ {
		resource := items.Index(i).Addr().Interface().(*unstructured.Unstructured)
		yamlData, err := toYAML(resource)
		if err != nil {
			log.Printf("Error serialising %s to YAML: %v\n", resourceType, err)
			continue
		}

		var fileData []byte
		if isSecret {
			if privateKey == "" {
				log.Printf("No private key provided for encryption of %s\n", resource.GetName())
				continue
			}
			fileData, err = encryptYAML([]byte(yamlData), privateKey)
			if err != nil {
				log.Printf("Error encrypting YAML for %s: %v\n", resource.GetName(), err)
				continue
			}
		} else {
			fileData = []byte(yamlData)
		}

		resourceName := resource.GetName()
		fileName := fmt.Sprintf("%s/%s/%s.yaml", localBackupDir, resourceType, resourceName)
		if err := os.WriteFile(fileName, fileData, 0644); err != nil {
			log.Printf("Error writing YAML to file for %s: %v\n", resourceName, err)
			continue
		}
		writtenFiles[fileName] = true
		yamlFileCount++
		if isSecret {
			log.Printf("Wrote encrypted %s YAML for %s to file %s\n", resourceType, resourceName, fileName)
		} else {
			log.Printf("Wrote %s YAML for %s to file %s\n", resourceType, resourceName, fileName)
		}
	}

	files, err := os.ReadDir(fmt.Sprintf("%s/%s", localBackupDir, resourceType))
	if err != nil {
		log.Printf("Error reading '%s/%s' directory: %v\n", localBackupDir, resourceType, err)
		return
	}

	for _, file := range files {
		filePath := fmt.Sprintf("%s/%s/%s", localBackupDir, resourceType, file.Name())
		if !writtenFiles[filePath] {
			err := os.Remove(filePath)
			if err != nil {
				log.Printf("Error removing old file %s: %v\n", file.Name(), err)
				continue
			}
			log.Printf("Removed old file %s\n", file.Name())
		}
	}

	if kmbMetrics != nil {
		kmbMetrics.setMetrics(resourceType, yamlFileCount)
	}
}

func toYAML(obj runtime.Object) (string, error) {
	yamlSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	var sb strings.Builder
	if err := yamlSerializer.Encode(obj, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func scheduleBackups(backupSchedule string, dynamicClient dynamic.Interface, localBackupDir string, kmbMetrics *kmbmetrics, privateKey string, backupResourcesYamlFile string, s3Configuration s3Config) {

	// schedule backups
	s := gocron.NewScheduler(time.UTC)
	job, err := s.Cron(backupSchedule).Do(performBackup, dynamicClient, localBackupDir, kmbMetrics, privateKey, backupResourcesYamlFile, s3Configuration)
	if err != nil {
		log.Fatalf("Error creating job: %v", err)
	}
	s.StartAsync()
	log.Printf("First backup scheduled: %v, backup schedule: %v", job.NextRun(), backupSchedule)

}

func performBackup(dynamicClient dynamic.Interface, localBackupDir string, kmbMetrics *kmbmetrics, privateKey string, backupResourcesYamlFile string, s3Configuration s3Config) {
	log.Printf("Starting backup\n")

	backupResources(dynamicClient, localBackupDir, kmbMetrics, privateKey, backupResourcesYamlFile, s3Configuration)

	log.Printf("Backup complete\n---\n")
}

// Initialise Prometheus metrics
func initialiseMetrics() kmbmetrics {
	var kmbMetrics kmbmetrics
	kmbMetrics.backupCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "backup_count",
			Help: "The amount of backups",
		},
		[]string{
			"backup_config_name",
			"backup_status",
		},
	)
	kmbMetrics.yamlFileCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "yaml_file_count",
			Help: "The number of YAML files backed up",
		},
		[]string{
			"resource_type",
		},
	)

	prometheus.MustRegister(kmbMetrics.backupCount)
	prometheus.MustRegister(kmbMetrics.yamlFileCount)

	return kmbMetrics
}

// Set Prometheus metrics values
func (kmbMetrics *kmbmetrics) setMetrics(resourceType string, yamlFileCount int) {
	log.Printf("Setting Prometheus metrics for resource type: %v\n", resourceType)

	// Set yamlFileCount metric for the resource type
	kmbMetrics.yamlFileCount.WithLabelValues(resourceType).Set(float64(yamlFileCount))
}

func encryptYAML(data []byte, privateKey string) ([]byte, error) {

	if privateKey == "" {
		err := fmt.Errorf("No private key provided for encryption")
		return nil, err
	}

	// Convert the privateKey string to an io.Reader
	privateKeyReader := strings.NewReader(privateKey)

	entityList, err := openpgp.ReadArmoredKeyRing(privateKeyReader)
	if err != nil {
		return nil, err
	}

	entity := entityList[0]

	// Encrypt the data
	buf := new(bytes.Buffer)
	w, err := openpgp.Encrypt(buf, []*openpgp.Entity{entity}, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	_, err = w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}

	// Encode the encrypted data in ASCII armor
	var armoredBuf bytes.Buffer
	armoredWriter, err := armor.Encode(&armoredBuf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, err
	}
	_, err = armoredWriter.Write(buf.Bytes())
	if err != nil {
		return nil, err
	}
	err = armoredWriter.Close()
	if err != nil {
		return nil, err
	}

	return armoredBuf.Bytes(), nil
}

// Helper function to get environment variable or default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		log.Printf("Environment variable %s found with value %s", key, value)
		return value
	}
	log.Printf("CLI flag and/or environment variable %s not set, using default value %s", key, defaultValue)
	return defaultValue
}
