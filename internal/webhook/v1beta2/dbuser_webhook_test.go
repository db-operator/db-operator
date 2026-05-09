/*
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

package v1beta2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kindarocksv1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
)

var _ = Describe("DbUser Webhook", func() {
	var (
		obj       *kindarocksv1beta2.DbUser
		oldObj    *kindarocksv1beta2.DbUser
		validator DbUserCustomValidator
	)

	BeforeEach(func() {
		obj = &kindarocksv1beta2.DbUser{}
		oldObj = &kindarocksv1beta2.DbUser{}
		validator = DbUserCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {})

	Context("When creating or updating DbUser under Validating Webhook", func() {})
})
