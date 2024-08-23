kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
    tr := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
    parsedToken, _ := parseJWT(tr.Spec.Token)

    expectedServiceAccount := "test-service-account"
    if tc.password == validK8sToken {
        if parsedToken["kubernetes.io"].(map[string]interface{})["serviceaccount"].(map[string]interface{})["name"] == expectedServiceAccount {
            return true, &authenticationv1.TokenReview{
                Status: authenticationv1.TokenReviewStatus{
                    Authenticated: true,
                },
            }, nil
        }
    }

    return true, &authenticationv1.TokenReview{
        Status: authenticationv1.TokenReviewStatus{
            Authenticated: false,
            Error:         "Invalid token",
        },
    }, nil
})
