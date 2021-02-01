
apply:
	terraform init
	terraform apply -auto-approve
	terraform output -raw k0s_cluster | go run ../../main.go apply --config -

destroy:
	terraform destroy -auto-approve

kubeconfig:
	terraform output -raw k0s_cluster | go run ../../main.go kubeconfig --config -
	