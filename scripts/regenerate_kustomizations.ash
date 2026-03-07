#!/usr/bin/env bash
set -e
if [[ ! -z $(git status --porcelain) ]]; then
  echo "Please add all you changes to git, the script is overwriting code in the repo"
  exit 1
fi

CURRENT_DIR=$(pwd)

WORKDIR=$(mktemp -d)
cd "${WORKDIR}"

echo "Workdir is ${WORKDIR}"

go mod init github.com/db-operator/db-operator/v2

kubebuilder init  --project-name db-operator --domain kinda.rocks
kubebuilder create api --kind DbInstance --version v1alpha1 --plural dbinstances --resource --controller
kubebuilder create api --kind Database --version v1alpha1 --plural databases --resource --controller --namespaced

kubebuilder create api --kind DbInstance --version v1beta1 --plural dbinstances --resource --controller=false
kubebuilder create api --kind Database --version v1beta1 --plural databases --resource --namespaced --controller=false
kubebuilder create api --kind DbUser --version v1beta1 --plural dbusers --resource --controller

kubebuilder create webhook --kind Database --version v1beta1 --programmatic-validation --defaulting --conversion
kubebuilder create webhook --kind DbInstance --version v1beta1 --programmatic-validation --defaulting --conversion
kubebuilder create webhook --kind DbUser --version v1beta1 --programmatic-validation --defaulting

cd "${CURRENT_DIR}"

rm -rf ./config

cp -r "${WORKDIR}/config" .
make manifests
