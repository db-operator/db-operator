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

package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindarocksv1 "github.com/db-operator/db-operator/v2/api/v1"
)

var _ = Describe("DbInstance Webhook", func() {
	var (
		obj       *kindarocksv1.DbInstance
		oldObj    *kindarocksv1.DbInstance
		validator DbInstanceCustomValidator
		defaulter DbInstanceCustomDefaulter
	)

	BeforeEach(func() {
		obj = &kindarocksv1.DbInstance{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       kindarocksv1.DbInstanceSpec{},
			Status:     kindarocksv1.DbInstanceStatus{},
		}
		oldObj = &kindarocksv1.DbInstance{}
		validator = DbInstanceCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = DbInstanceCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("When creating DbInstance under Defaulting Webhook", func() {
		// TODO (user): Add logic for defaulting webhooks
		// Example:
		// It("Should apply defaults when a required field is empty", func() {
		//     By("simulating a scenario where defaults should be applied")
		//     obj.SomeFieldWithDefault = ""
		//     By("calling the Default method to apply defaults")
		//     defaulter.Default(ctx, obj)
		//     By("checking that the default values are set")
		//     Expect(obj.SomeFieldWithDefault).To(Equal("default_value"))
		// })
	})

	Context("When creating or updating DbInstance under Validating Webhook", func() {
		It("Should deny creation if getting a password from a CM", func() {
			By("simulating an invalid creation scenario")
			ns := "namespace"
			name := "name"
			key := "key"
			obj := obj.DeepCopy()
			obj.Spec.Auth = &kindarocksv1.DbInstanceAuth{
				Password: &kindarocksv1.ValueSource{
					ValueFrom: &kindarocksv1.ValueFrom{
						ConfigMapKeyRef: &kindarocksv1.SecretOrCMRef{
							Namespace: &ns,
							Name:      &name,
							Key:       &key,
						},
					},
				},
			}
			Expect(validator.ValidateCreate(GinkgoT().Context(), obj)).Error().To(HaveOccurred())
		})
		It("Should deny creation if reading a password directly from a value", func() {
			By("simulating an invalid creation scenario")
			key := "key"
			obj := obj.DeepCopy()
			obj.Spec.Auth = &kindarocksv1.DbInstanceAuth{
				Password: &kindarocksv1.ValueSource{
					Value: &key,
				},
			}
			Expect(validator.ValidateCreate(GinkgoT().Context(), obj)).Error().To(HaveOccurred())
		})
		It("Should allow creation if reading a password from a Secret", func() {
			By("simulating a valid creation scenario")
			ns := "namespace"
			name := "name"
			key := "key"
			obj := obj.DeepCopy()
			obj.Spec.Auth = &kindarocksv1.DbInstanceAuth{
				Password: &kindarocksv1.ValueSource{
					ValueFrom: &kindarocksv1.ValueFrom{
						SecretKeyRef: &kindarocksv1.SecretOrCMRef{
							Namespace: &ns,
							Name:      &name,
							Key:       &key,
						},
					},
				},
			}
			Expect(validator.ValidateCreate(GinkgoT().Context(), obj)).Error().To(Not(HaveOccurred()))
		})
		It("Should deny update if getting a password from a CM", func() {
			By("simulating an invalid creation scenario")
			ns := "namespace"
			name := "name"
			key := "key"
			obj := obj.DeepCopy()
			obj.Spec.Auth = &kindarocksv1.DbInstanceAuth{
				Password: &kindarocksv1.ValueSource{
					ValueFrom: &kindarocksv1.ValueFrom{
						ConfigMapKeyRef: &kindarocksv1.SecretOrCMRef{
							Namespace: &ns,
							Name:      &name,
							Key:       &key,
						},
					},
				},
			}
			Expect(validator.ValidateUpdate(GinkgoT().Context(), oldObj, obj)).Error().To(HaveOccurred())
		})
		It("Should deny update if reading a password directly from a value", func() {
			By("simulating an invalid creation scenario")
			key := "key"
			obj := obj.DeepCopy()
			obj.Spec.Auth = &kindarocksv1.DbInstanceAuth{
				Password: &kindarocksv1.ValueSource{
					Value: &key,
				},
			}
			Expect(validator.ValidateUpdate(GinkgoT().Context(), oldObj, obj)).Error().To(HaveOccurred())
		})
		It("Should allow update if reading a password from a Secret", func() {
			By("simulating a valid creation scenario")
			ns := "namespace"
			name := "name"
			key := "key"
			obj := obj.DeepCopy()
			obj.Spec.Auth = &kindarocksv1.DbInstanceAuth{
				Password: &kindarocksv1.ValueSource{
					ValueFrom: &kindarocksv1.ValueFrom{
						SecretKeyRef: &kindarocksv1.SecretOrCMRef{
							Namespace: &ns,
							Name:      &name,
							Key:       &key,
						},
					},
				},
			}
			Expect(validator.ValidateUpdate(GinkgoT().Context(), oldObj, obj)).Error().To(Not(HaveOccurred()))
		})
	})

	Context("When deleting DbInstance under Validating Webhook", func() {
		It("Should simply remove a DbInstance", func() {
			By("simulating a deletion creation scenario")
			obj := obj.DeepCopy()
			warn, err := validator.ValidateDelete(GinkgoT().Context(), obj)
			Expect(err).To(Not(HaveOccurred()))
			Expect(warn).To(BeEmpty())
		})
		It("Should notify about Databases and remove a DbInstane", func() {
			By("simulating a deletion creation scenario")
			obj := obj.DeepCopy()
			obj.Status = kindarocksv1.DbInstanceStatus{
				ServerStatus: &kindarocksv1.DbInstanceServerStatus{
					ManagedDatabasesCount: 1,
				},
			}
			warn, err := validator.ValidateDelete(GinkgoT().Context(), obj)
			Expect(err).To(Not(HaveOccurred()))
			Expect(warn).To(Not(BeEmpty()))
		})
	})
})
