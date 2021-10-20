package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
)

const testKubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ZHNqaA==
    server: https://api.member-cluster-0
  name: member-cluster-0
- cluster:
    certificate-authority-data: ZHNqaA==
    server: https://api.member-cluster-1
  name: member-cluster-1
- cluster:
    certificate-authority-data: ZHNqaA==
    server: https://api.member-cluster-2
  name: member-cluster-2
contexts:
- context:
    cluster: member-cluster-0
    namespace: citi
    user: member-cluster-0
  name: member-cluster-0
- context:
    cluster: member-cluster-1
    namespace: citi
    user: member-cluster-1
  name: member-cluster-1
- context:
    cluster: member-cluster-2
    namespace: citi
    user: member-cluster-2
  name: member-cluster-2
current-context: member-cluster-0
kind: Config
preferences: {}
users:
- name: member-cluster-0
  user:
    client-certificate-data: ZHNqaA==
    client-key-data: ZHNqaA==
`

func testFlags(t *testing.T, cleanup bool) flags {
	memberClusters := []string{"member-cluster-0", "member-cluster-1", "member-cluster-2"}

	kubeconfig, err := clientcmd.Load([]byte(testKubeconfig))
	assert.NoError(t, err)

	memberClusterApiServerUrls, err := getMemberClusterApiServerUrls(kubeconfig, memberClusters)
	assert.NoError(t, err)

	return flags{
		memberClusterApiServerUrls: memberClusterApiServerUrls,
		memberClusters:             memberClusters,
		serviceAccount:             "test-service-account",
		centralCluster:             "central-cluster",
		memberClusterNamespace:     "member-namespace",
		centralClusterNamespace:    "central-namespace",
		cleanup:                    cleanup,
		clusterScoped:              false,
	}

}

func TestNamespaces_GetsCreated_WhenTheyDoNotExit(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)

	assertMemberClusterNamespacesExist(t, clientMap, flags)
	assertCentralClusterNamespacesExist(t, clientMap, flags)
}

func TestExistingNamespaces_DoNotCause_AlreadyExistsErrors(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags, namespaceResourceType)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)

	assertMemberClusterNamespacesExist(t, clientMap, flags)
	assertCentralClusterNamespacesExist(t, clientMap, flags)
}

func TestServiceAccount_GetsCreate_WhenTheyDoNotExit(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertServiceAccountsExist(t, clientMap, flags)
}

func TestExistingServiceAccounts_DoNotCause_AlreadyExistsErrors(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags, serviceAccountResourceType)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertServiceAccountsExist(t, clientMap, flags)
}

func TestRoles_GetsCreated_WhenTheyDoesNotExit(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertMemberRolesExist(t, clientMap, flags)
}

func TestExistingRoles_DoNotCause_AlreadyExistsErrors(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags, roleResourceType)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertMemberRolesExist(t, clientMap, flags)
}

func TestClusterRoles_DoNotGetCreated_WhenNotSpecified(t *testing.T) {
	flags := testFlags(t, false)
	flags.clusterScoped = false

	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertMemberRolesExist(t, clientMap, flags)
	assertCentralRolesExist(t, clientMap, flags)
}

func TestClusterRoles_GetCreated_WhenSpecified(t *testing.T) {
	flags := testFlags(t, false)
	flags.clusterScoped = true

	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertMemberRolesDoNotExist(t, clientMap, flags)
	assertMemberClusterRolesExist(t, clientMap, flags)
}

func TestCentralCluster_GetsRegularRoleCreated_WhenClusterScoped_IsSpecified(t *testing.T) {
	flags := testFlags(t, false)
	flags.clusterScoped = true

	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
}

func TestCentralCluster_GetsRegularRoleCreated_WhenNonClusterScoped_IsSpecified(t *testing.T) {
	flags := testFlags(t, false)
	flags.clusterScoped = false

	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))

	assert.NoError(t, err)
	assertCentralRolesExist(t, clientMap, flags)
}

func TestPerformCleanup(t *testing.T) {
	flags := testFlags(t, true)
	flags.clusterScoped = true

	clientMap := getClientResources(flags)
	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
	assert.NoError(t, err)

	t.Run("Resources get created with labels", func(t *testing.T) {
		assertMemberClusterRolesExist(t, clientMap, flags)
		assertMemberClusterNamespacesExist(t, clientMap, flags)
		assertCentralClusterNamespacesExist(t, clientMap, flags)
		assertServiceAccountsExist(t, clientMap, flags)
	})

	err = performCleanup(clientMap, flags)
	assert.NoError(t, err)

	t.Run("Resources with labels are removed", func(t *testing.T) {
		assertMemberRolesDoNotExist(t, clientMap, flags)
		assertMemberClusterRolesDoNotExist(t, clientMap, flags)
		assertCentralRolesDoNotExist(t, clientMap, flags)
	})

	t.Run("Namespaces are preserved", func(t *testing.T) {
		assertMemberClusterNamespacesExist(t, clientMap, flags)
		assertCentralClusterNamespacesExist(t, clientMap, flags)
	})

}

func TestCreateKubeConfig_IsComposedOf_ServiceAccountTokens_InAllClusters(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)

	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
	assert.NoError(t, err)

	kubeConfig, err := readKubeConfig(clientMap[flags.centralCluster], flags.centralClusterNamespace)
	assert.NoError(t, err)

	assert.Equal(t, "Config", kubeConfig.Kind)
	assert.Equal(t, "v1", kubeConfig.ApiVersion)
	assert.Len(t, kubeConfig.Contexts, len(flags.memberClusters))
	assert.Len(t, kubeConfig.Clusters, len(flags.memberClusters))

	for i, kubeConfigCluster := range kubeConfig.Clusters {
		assert.Equal(t, flags.memberClusters[i], kubeConfigCluster.Name, "Name of cluster should be set to the member clusters.")
		expectedCaBytes, err := readSecretKey(clientMap[flags.memberClusters[i]], fmt.Sprintf("%s-token", flags.serviceAccount), flags.memberClusterNamespace, "ca.crt")

		assert.NoError(t, err)
		assert.Contains(t, string(expectedCaBytes), flags.memberClusters[i])
		assert.Equal(t, 0, bytes.Compare(expectedCaBytes, kubeConfigCluster.Cluster.CertificateAuthorityData), "CA should be read from Service Account token Secret.")
		assert.Equal(t, fmt.Sprintf("https://api.%s", flags.memberClusters[i]), kubeConfigCluster.Cluster.Server, "Server should be correctly configured based on cluster name.")
	}

	for i, user := range kubeConfig.Users {
		tokenBytes, err := readSecretKey(clientMap[flags.memberClusters[i]], fmt.Sprintf("%s-token", flags.serviceAccount), flags.memberClusterNamespace, "token")
		assert.NoError(t, err)
		assert.Equal(t, flags.memberClusters[i], user.Name, "User name should be the name of the cluster.")
		assert.Equal(t, string(tokenBytes), user.User.Token, "Token from the service account secret should be set.")
	}

}

func TestKubeConfigSecret_IsCreated_InCentralCluster(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)

	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
	assert.NoError(t, err)

	centralClusterClient := clientMap[flags.centralCluster]
	kubeConfigSecret, err := centralClusterClient.CoreV1().Secrets(flags.centralClusterNamespace).Get(context.TODO(), kubeConfigSecretName, metav1.GetOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, kubeConfigSecret)
}

func TestKubeConfigSecret_IsNotCreated_InMemberClusters(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)

	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
	assert.NoError(t, err)

	for _, memberCluster := range flags.memberClusters {
		memberClient := clientMap[memberCluster]
		kubeConfigSecret, err := memberClient.CoreV1().Secrets(flags.centralClusterNamespace).Get(context.TODO(), kubeConfigSecretName, metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err))
		assert.Nil(t, kubeConfigSecret)
	}
}

func TestChangingOneServiceAccountToken_ChangesOnlyThatEntry_InKubeConfig(t *testing.T) {
	flags := testFlags(t, false)
	clientMap := getClientResources(flags)

	err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
	assert.NoError(t, err)

	kubeConfigBefore, err := readKubeConfig(clientMap[flags.centralCluster], flags.centralClusterNamespace)
	assert.NoError(t, err)

	firstClusterClient := clientMap[flags.memberClusters[0]]

	// simulate a service account token changing, re-running the script should leave the other clusters unchanged.
	newServiceAccountToken := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", flags.serviceAccount),
			Namespace: flags.memberClusterNamespace,
		},
		Data: map[string][]byte{
			"token":  []byte("new-token-data"),
			"ca.crt": []byte("new-ca-crt"),
		},
	}

	_, err = firstClusterClient.CoreV1().Secrets(flags.memberClusterNamespace).Update(context.TODO(), &newServiceAccountToken, metav1.UpdateOptions{})
	assert.NoError(t, err)

	err = ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
	assert.NoError(t, err)

	kubeConfigAfter, err := readKubeConfig(clientMap[flags.centralCluster], flags.centralClusterNamespace)
	assert.NoError(t, err)

	assert.NotEqual(t, kubeConfigBefore.Users[0], kubeConfigAfter.Users[0], "Cluster 0 users should have been modified.")
	assert.NotEqual(t, kubeConfigBefore.Clusters[0], kubeConfigAfter.Clusters[0], "Cluster 1 clusters should have been modified")

	assert.Equal(t, "new-token-data", kubeConfigAfter.Users[0].User.Token, "first user token should have been updated.")
	assert.Equal(t, []byte("new-ca-crt"), kubeConfigAfter.Clusters[0].Cluster.CertificateAuthorityData, "CA for cluster 0 should have been updated.")

	assert.Equal(t, kubeConfigBefore.Users[1], kubeConfigAfter.Users[1], "Cluster 1 users should have remained unchanged")
	assert.Equal(t, kubeConfigBefore.Clusters[1], kubeConfigAfter.Clusters[1], "Cluster 1 clusters should have remained unchanged")

	assert.Equal(t, kubeConfigBefore.Users[2], kubeConfigAfter.Users[2], "Cluster 2 users should have remained unchanged")
	assert.Equal(t, kubeConfigBefore.Clusters[2], kubeConfigAfter.Clusters[2], "Cluster 2 clusters should have remained unchanged")
}

func TestGetMemberClusterApiServerUrls(t *testing.T) {
	t.Run("Test comma separated string returns correct values", func(t *testing.T) {
		kubeconfig, err := clientcmd.Load([]byte(testKubeconfig))
		assert.NoError(t, err)

		apiUrls, err := getMemberClusterApiServerUrls(kubeconfig, []string{"member-cluster-0", "member-cluster-1", "member-cluster-2"})
		assert.Nil(t, err)
		assert.Len(t, apiUrls, 3)
		assert.Equal(t, apiUrls[0], "https://api.member-cluster-0")
		assert.Equal(t, apiUrls[1], "https://api.member-cluster-1")
		assert.Equal(t, apiUrls[2], "https://api.member-cluster-2")
	})

	t.Run("Test missing cluster lookup returns error", func(t *testing.T) {
		kubeconfig, err := clientcmd.Load([]byte(testKubeconfig))
		assert.NoError(t, err)

		_, err = getMemberClusterApiServerUrls(kubeconfig, []string{"member-cluster-0", "member-cluster-1", "member-cluster-missing"})
		assert.Error(t, err)
	})
}

func TestMemberClusterUris(t *testing.T) {
	t.Run("Uses server values set in flags", func(t *testing.T) {
		flags := testFlags(t, false)
		flags.memberClusterApiServerUrls = []string{"cluster1-url", "cluster2-url", "cluster3-url"}
		clientMap := getClientResources(flags)

		err := ensureMultiClusterResources(flags, getFakeClientFunction(clientMap, nil))
		assert.NoError(t, err)

		kubeConfig, err := readKubeConfig(clientMap[flags.centralCluster], flags.centralClusterNamespace)
		assert.NoError(t, err)

		for i, c := range kubeConfig.Clusters {
			assert.Equal(t, flags.memberClusterApiServerUrls[i], c.Cluster.Server)
		}

		assert.NoError(t, err)
	})
}

// assertMemberClusterNamespacesExist asserts the Namespace in the member clusters exists.
func assertMemberClusterNamespacesExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	for _, clusterName := range flags.memberClusters {
		client := clientMap[clusterName]
		ns, err := client.CoreV1().Namespaces().Get(context.TODO(), flags.memberClusterNamespace, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, ns)
		assert.Equal(t, flags.memberClusterNamespace, ns.Name)
		assert.Equal(t, ns.Labels, multiClusterLabels())
	}
}

// assertCentralClusterNamespacesExist asserts the Namespace in the central cluster exists..
func assertCentralClusterNamespacesExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	client := clientMap[flags.centralCluster]
	ns, err := client.CoreV1().Namespaces().Get(context.TODO(), flags.centralClusterNamespace, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ns)
	assert.Equal(t, flags.centralClusterNamespace, ns.Name)
	assert.Equal(t, ns.Labels, multiClusterLabels())
}

// assertServiceAccountsAreCorrect asserts the ServiceAccounts are created as expected.
func assertServiceAccountsExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	for _, clusterName := range flags.memberClusters {
		client := clientMap[clusterName]
		sa, err := client.CoreV1().ServiceAccounts(flags.memberClusterNamespace).Get(context.TODO(), flags.serviceAccount, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, sa)
		assert.Equal(t, flags.serviceAccount, sa.Name)
		assert.Equal(t, sa.Labels, multiClusterLabels())
	}

	client := clientMap[flags.centralCluster]
	sa, err := client.CoreV1().ServiceAccounts(flags.centralClusterNamespace).Get(context.TODO(), flags.serviceAccount, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, sa)
	assert.Equal(t, flags.serviceAccount, sa.Name)
	assert.Equal(t, sa.Labels, multiClusterLabels())
}

// assertMemberClusterRolesExist should be used when member cluster cluster roles should exist.
func assertMemberClusterRolesExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	assertClusterRoles(t, clientMap, flags, true, memberCluster)
}

// assertMemberClusterRolesDoNotExist should be used when member cluster cluster roles should not exist.
func assertMemberClusterRolesDoNotExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	assertClusterRoles(t, clientMap, flags, false, centralCluster)
}

// assertClusterRoles should be used to assert the existence of member cluster cluster roles. The boolean
// shouldExist should be true for roles existing, and false for cluster roles not existing.
func assertClusterRoles(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags, shouldExist bool, clusterType clusterType) {
	var expectedClusterRole rbacv1.ClusterRole
	if clusterType == centralCluster {
		expectedClusterRole = buildCentralEntityClusterRole()
	} else {
		expectedClusterRole = buildMemberEntityClusterRole()
	}

	for _, clusterName := range flags.memberClusters {
		client := clientMap[clusterName]
		role, err := client.RbacV1().ClusterRoles().Get(context.TODO(), expectedClusterRole.Name, metav1.GetOptions{})
		if shouldExist {
			assert.NoError(t, err)
			assert.NotNil(t, role)
			assert.Equal(t, expectedClusterRole, *role)
		} else {
			assert.Error(t, err)
			assert.Nil(t, role)
		}
	}

	clusterRole, err := clientMap[flags.centralCluster].RbacV1().ClusterRoles().Get(context.TODO(), expectedClusterRole.Name, metav1.GetOptions{})
	if shouldExist {
		assert.Nil(t, err)
		assert.NotNil(t, clusterRole)
	} else {
		assert.Error(t, err)
	}
}

// assertMemberRolesExist should be used when member cluster roles should exist.
func assertMemberRolesExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	assertMemberRolesAreCorrect(t, clientMap, flags, true)
}

// assertMemberRolesDoNotExist should be used when member cluster roles should not exist.
func assertMemberRolesDoNotExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	assertMemberRolesAreCorrect(t, clientMap, flags, false)
}

// assertMemberRolesAreCorrect should be used to assert the existence of member cluster roles. The boolean
// shouldExist should be true for roles existing, and false for roles not existing.
func assertMemberRolesAreCorrect(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags, shouldExist bool) {
	expectedRole := buildMemberEntityRole(flags.memberClusterNamespace)

	for _, clusterName := range flags.memberClusters {
		client := clientMap[clusterName]
		role, err := client.RbacV1().Roles(flags.memberClusterNamespace).Get(context.TODO(), expectedRole.Name, metav1.GetOptions{})
		if shouldExist {
			assert.NoError(t, err)
			assert.NotNil(t, role)
			assert.Equal(t, expectedRole, *role)
		} else {
			assert.Error(t, err)
			assert.Nil(t, role)
		}
	}
}

// assertCentralRolesExist should be used when central cluster roles should exist.
func assertCentralRolesExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	assertCentralRolesAreCorrect(t, clientMap, flags, true)
}

// assertCentralRolesDoNotExist should be used when central cluster roles should not exist.
func assertCentralRolesDoNotExist(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags) {
	assertCentralRolesAreCorrect(t, clientMap, flags, false)
}

// assertCentralRolesAreCorrect should be used to assert the existence of central cluster roles. The boolean
// shouldExist should be true for roles existing, and false for roles not existing.
func assertCentralRolesAreCorrect(t *testing.T, clientMap map[string]kubernetes.Interface, flags flags, shouldExist bool) {
	client := clientMap[flags.centralCluster]

	// should never have a cluster role
	clusterRole := buildCentralEntityClusterRole()
	cr, err := client.RbacV1().ClusterRoles().Get(context.TODO(), clusterRole.Name, metav1.GetOptions{})

	assert.True(t, errors.IsNotFound(err))
	assert.Nil(t, cr)

	expectedRole := buildCentralEntityRole(flags.centralClusterNamespace)
	role, err := client.RbacV1().Roles(flags.centralClusterNamespace).Get(context.TODO(), expectedRole.Name, metav1.GetOptions{})

	if shouldExist {
		assert.NoError(t, err, "should always create a role for central cluster")
		assert.NotNil(t, role)
		assert.Equal(t, expectedRole, *role)
	} else {
		assert.Error(t, err)
		assert.Nil(t, role)
	}
}

// resourceType indicates a type of resource that is created during the tests.
type resourceType string

var (
	serviceAccountResourceType resourceType = "ServiceAccount"
	namespaceResourceType      resourceType = "Namespace"
	roleBindingResourceType    resourceType = "RoleBinding"
	roleResourceType           resourceType = "Role"
)

// createResourcesForCluster returns the resources specified based on the provided resourceTypes.
// this function is used to populate subsets of resources for the unit tests.
func createResourcesForCluster(centralCluster bool, flags flags, clusterName string, resourceTypes ...resourceType) []runtime.Object {
	var namespace = flags.memberClusterNamespace
	if centralCluster {
		namespace = flags.centralCluster
	}

	resources := make([]runtime.Object, 0)

	// always create the service account token secret as this gets created by
	// kubernetes, we can just assume it is always there for tests.
	resources = append(resources, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", flags.serviceAccount),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"ca.crt": []byte(fmt.Sprintf("ca-cert-data-%s", clusterName)),
			"token":  []byte(fmt.Sprintf("%s-token-data", clusterName)),
		},
	})

	if containsResourceType(resourceTypes, namespaceResourceType) {
		resources = append(resources, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespace,
				Labels: multiClusterLabels(),
			},
		})
	}

	if containsResourceType(resourceTypes, serviceAccountResourceType) {
		resources = append(resources, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:   flags.serviceAccount,
				Labels: multiClusterLabels(),
			},
			Secrets: []corev1.ObjectReference{
				{
					Name:      flags.serviceAccount + "-token",
					Namespace: namespace,
				},
			},
		})
	}

	if containsResourceType(resourceTypes, roleResourceType) {
		role := buildMemberEntityRole(namespace)
		resources = append(resources, &role)
	}

	if containsResourceType(resourceTypes, roleBindingResourceType) {
		role := buildMemberEntityRole(namespace)
		roleBinding := buildRoleBinding(role, namespace)
		resources = append(resources, &roleBinding)
	}

	return resources
}

// getClientResources returns a map of cluster name to fake.Clientset
func getClientResources(flags flags, resourceTypes ...resourceType) map[string]kubernetes.Interface {
	clientMap := make(map[string]kubernetes.Interface)

	for _, clusterName := range flags.memberClusters {
		resources := createResourcesForCluster(false, flags, clusterName, resourceTypes...)
		clientMap[clusterName] = fake.NewSimpleClientset(resources...)
	}
	resources := createResourcesForCluster(true, flags, flags.centralCluster, resourceTypes...)
	clientMap[flags.centralCluster] = fake.NewSimpleClientset(resources...)

	return clientMap
}

// getFakeClientFunction returns a function which will return the fake.Clientset corresponding to the given cluster.
func getFakeClientFunction(clientResources map[string]kubernetes.Interface, err error) func(clusterName, kubeConfigPath string) (kubernetes.Interface, error) {
	return func(clusterName, kubeConfigPath string) (kubernetes.Interface, error) {
		return clientResources[clusterName], err
	}
}

// containsResourceType returns true if r is in resourceTypes, otherwise false.
func containsResourceType(resourceTypes []resourceType, r resourceType) bool {
	for _, rt := range resourceTypes {
		if rt == r {
			return true
		}
	}
	return false
}

// readSecretKey reads a key from a Secret in the given namespace with the given name.
func readSecretKey(client kubernetes.Interface, secretName, namespace, key string) ([]byte, error) {
	tokenSecret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return tokenSecret.Data[key], nil
}

// readKubeConfig reads the KubeConfig file from the secret in the given cluster and namespace.
func readKubeConfig(client kubernetes.Interface, namespace string) (KubeConfigFile, error) {
	kubeConfigSecret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), kubeConfigSecretName, metav1.GetOptions{})
	if err != nil {
		return KubeConfigFile{}, err
	}

	kubeConfigBytes := kubeConfigSecret.Data[kubeConfigSecretKey]
	result := KubeConfigFile{}
	if err := yaml.Unmarshal(kubeConfigBytes, &result); err != nil {
		return KubeConfigFile{}, err
	}

	return result, nil
}
