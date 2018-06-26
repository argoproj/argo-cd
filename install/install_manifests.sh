kubectl apply -f manifests/00_namespace.yaml
kubectl apply -f manifests/01a_application-crd.yaml			
kubectl apply -f manifests/01b_appproject-crd.yaml				
#02a and 02b are just example manifests
kubectl apply -f manifests/02c_argocd-rbac-cm.yaml				
kubectl apply -f manifests/03a_application-controller-sa.yaml		
kubectl apply -f manifests/03b_application-controller-role.yaml		
kubectl apply -f manifests/03c_application-controller-rolebinding.yaml	
kubectl apply -f manifests/03d_application-controller-deployment.yaml	
kubectl apply -f manifests/04a_argocd-server-sa.yaml			
kubectl apply -f manifests/04b_argocd-server-role.yaml			
kubectl apply -f manifests/04c_argocd-server-rolebinding.yaml	
kubectl apply -f manifests/04d_argocd-server-deployment.yaml
kubectl apply -f manifests/04e_argocd-server-service.yaml
kubectl apply -f manifests/05a_argocd-repo-server-deployment.yaml
kubectl apply -f manifests/05b_argocd-repo-server-service.yaml