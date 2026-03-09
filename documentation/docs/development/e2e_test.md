---
icon: lucide/book-open-check
---

# E2e tests

E2e tests should cover user workflow. They expect the operator to be already deployed in the cluster, so they can just execute commands that a user would execute and then check whether the operator is behaving correctly.

## Code structure

E2e tests are developed as a separate go package in the `./test` directory. They should not depend on any types described in the db-operator, instead they should always rely on the Kubernetes api.

## Tools

You will need to have the following tools to be able to execute tests:
- go
- kubectl
- kustomize

You will also need to have a running cluster with db-operator installed.

## How it works

E2e tests are creating resources that you can find in the `./test/manifests` folder, so to use 
