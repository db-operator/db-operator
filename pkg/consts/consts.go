/*
 * Copyright 2023 DB-Operator Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package consts

// This package exists to avoid cycle import. Put here consts that are used across packages

// Database Related Consts
const (
	POSTGRES_DB       = "POSTGRES_DB"
	POSTGRES_USER     = "POSTGRES_USER"
	POSTGRES_PASSWORD = "POSTGRES_PASSWORD"
	MYSQL_DB          = "DB"
	MYSQL_USER        = "USER"
	MYSQL_PASSWORD    = "PASSWORD"
)

// Database engines
const (
	ENGINE_POSTGRES = "postgres"
	ENGINE_MYSQL    = "mysql"
)

// SSL modes
const (
	SSL_DISABLED  = "disabled"
	SSL_REQUIRED  = "required"
	SSL_VERIFY_CA = "verify_ca"
)

// Kubernetes Annotations
const (
	DBINSTANCE_ALLOW_MIGRATION    = "kinda.rocks/db-instance-allow-migration"
	TEMPLATE_ANNOTATION_KEY       = "kinda.rocks/db-operator-templated-keys"
	SECRET_FORCE_RECONCILE        = "kinda.rocks/secret-force-reconcile"
	DATABASE_FORCE_FULL_RECONCILE = "kinda.rocks/db-force-full-reconcile"
	USED_OBJECTS                  = "kinda.rocks/used-objects"
	// This annotation should be used, when a DbUser is not allowed to log in
	// with password
	RDS_IAM_IMPERSONATE_WORKAROUND = "kinda.rocks/rds-iam-impersonate"
	// On instances where the admin is not a super user, it might not be able
	// to drop owned by user, so we need to grant the user to the admin,
	// But it's not possible on the AWS instances with the rds_iam role,
	// because then admins are not able to log in with a password anymore
	GRANT_TO_ADMIN_ON_DELETE = "kinda.rocks/grant-to-admin-on-delete"
)

// Kubernetes Labels
const (
	MANAGED_BY_LABEL_KEY   = "app.kubernetes.io/managed-by"
	MANAGED_BY_LABEL_VALUE = "db-operator"
	USED_BY_KIND_LABEL_KEY = "kinda.rocks/used-by-kind"
	USED_BY_NAME_LABEL_KEY = "kinda.rocks/used-by-name"
)

// Privileges

const ALL_PRIVILEGES = "ALL PRIVILEGES"
