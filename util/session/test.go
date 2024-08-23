var kubeClientset kubernetes.Interface
			if tc.isK8sToken {
				kubeClientset = fake.NewSimpleClientset()

				if tc.password == validK8sToken {
					// Mock TokenReview response
					kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
						tr := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
						if tr.Spec.Token == validK8sToken { // Use valid JWT
							return true, &authenticationv1.TokenReview{
								Status: authenticationv1.TokenReviewStatus{
									Authenticated: true,
								},
							}, nil
						}
						return true, &authenticationv1.TokenReview{
							Status: authenticationv1.TokenReviewStatus{
								Authenticated: false,
								Error:         "Invalid token",
							},
						}, nil
					})

					// Override the password with the fake token
					tc.password = validK8sToken
				}
			} else {
				kubeClientset = nil
			}
