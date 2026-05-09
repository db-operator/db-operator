package controllers

import (
	"testing"

	kindav1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestIsWatchedNamespaceClusterWide(t *testing.T) {
	obj := &kindav1beta2.Database{ObjectMeta: metav1.ObjectMeta{Namespace: "db"}}

	assert.True(t, isWatchedNamespace([]string{""}, obj))
	assert.True(t, isWatchedNamespace([]string{}, obj))
}

func TestIsWatchedNamespaceScoped(t *testing.T) {
	dbObj := &kindav1beta2.Database{ObjectMeta: metav1.ObjectMeta{Namespace: "db-ns"}}
	secretObj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "sec-ns"}}
	configMapObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "cfg-ns"}}

	assert.True(t, isWatchedNamespace([]string{"db-ns", "other"}, dbObj))
	assert.False(t, isWatchedNamespace([]string{"other"}, dbObj))
	assert.True(t, isWatchedNamespace([]string{"sec-ns"}, secretObj))
	assert.False(t, isWatchedNamespace([]string{"other"}, secretObj))
	assert.False(t, isWatchedNamespace([]string{"cfg-ns"}, configMapObj))
}

func TestIsDatabase(t *testing.T) {
	dbObj := &kindav1beta2.Database{}
	secretObj := &corev1.Secret{}

	assert.True(t, isDatabase(dbObj))
	assert.False(t, isDatabase(secretObj))
}

func TestIsObjectUpdatedDatabase(t *testing.T) {
	dbOld := &kindav1beta2.Database{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
	dbNewSame := &kindav1beta2.Database{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
	dbNewDiff := &kindav1beta2.Database{ObjectMeta: metav1.ObjectMeta{Generation: 2}}

	assert.False(t, isObjectUpdated(event.UpdateEvent{ObjectOld: dbOld, ObjectNew: dbNewSame}))
	assert.True(t, isObjectUpdated(event.UpdateEvent{ObjectOld: dbOld, ObjectNew: dbNewDiff}))
}

func TestIsObjectUpdatedSecretLabel(t *testing.T) {
	oldSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "s1"}}
	newSecretMissingLabel := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "s1"}}
	newSecretWithLabel := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ns",
		Name:      "s1",
		Labels: map[string]string{
			consts.USED_BY_NAME_LABEL_KEY: "db1",
		},
	}}

	assert.False(t, isObjectUpdated(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: newSecretMissingLabel}))
	assert.True(t, isObjectUpdated(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: newSecretWithLabel}))
}

func TestIsObjectUpdatedNilObjects(t *testing.T) {
	evtNilOld := event.UpdateEvent{ObjectOld: nil, ObjectNew: &corev1.Secret{}}
	evtNilNew := event.UpdateEvent{ObjectOld: &corev1.Secret{}, ObjectNew: nil}

	assert.False(t, isObjectUpdated(evtNilOld))
	assert.False(t, isObjectUpdated(evtNilNew))
}

func TestIsObjectUpdatedUnknownObject(t *testing.T) {
	oldCfg := &corev1.ConfigMap{}
	newCfg := &corev1.ConfigMap{}

	assert.False(t, isObjectUpdated(event.UpdateEvent{ObjectOld: oldCfg, ObjectNew: newCfg}))
}
