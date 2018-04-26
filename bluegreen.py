#! /usr/bin/env python3

import kubernetes
import base64
import os

from kubernetes import client, config

# configuration = kubernetes.client.Configuration()
# configuration.api_key['authorization'] = 'YOUR_API_KEY'
# # Uncomment below to setup prefix (e.g. Bearer) for API key, if needed
# # configuration.api_key_prefix['authorization'] = 'Bearer'

# # create an instance of the API class
# api_instance = kubernetes.client.ExtensionsV1beta1Api(kubernetes.client.ApiClient(configuration))


# Configs can be set in Configuration class directly or using helper utility
config.load_kube_config()

def read_service(namespace: str, name: str):
    """ Retrieve a service from Kubernetes.

    Parameters
    ----------
    namespace : str
        The namespace of a service to retrieve.
    name : str
        The name of a service to retrieve.

    Returns
    -------
    out : object
        An object containing information about the requested service,
        or `None` if no information could be found.

    Raises
    ------
    kubernetes.client.rest.ApiException
        If an exception reason other than `Not Found` occurs.

    """
    v1 = client.CoreV1Api()
    try:
        return v1.read_namespaced_service(name, namespace)
    except kubernetes.client.rest.ApiException as e:
        if e.status != 404:
            raise e
        return None

def patch_service(namespace: str, name: str, body: dict):
    """ Retrieve a service from Kubernetes.

    Parameters
    ----------
    namespace : str
        The namespace of a service to retrieve.
    name : str
        The name of a service to retrieve.
    body : dict
        Attributes to patch.

    Returns
    -------
    out : object
        An object containing information about the requested service,
        or `None` if no information could be found.

    Raises
    ------
    kubernetes.client.rest.ApiException
        If an exception occurs during patching.

    """
    v1 = client.CoreV1Api()
    try:
        return v1.patch_namespaced_service(name, namespace, body)
    except kubernetes.client.rest.ApiException as e:
        raise e

def read_deployment(namespace: str, name: str):
    """ Retrieve a deployment from Kubernetes.

    Parameters
    ----------
    namespace : str
        The namespace of a deployment to retrieve.
    name : str
        The name of a deployment to retrieve.

    Returns
    -------
    out : object
        An object containing information about the requested deployment,
        or `None` if no information could be found.

    Raises
    ------
    kubernetes.client.rest.ApiException
        If an exception reason other than `Not Found` occurs.

    """
    k8s_beta = client.ExtensionsV1beta1Api()
    try:
        return k8s_beta.read_namespaced_deployment(name, namespace)
    except kubernetes.client.rest.ApiException as e:
        if e.status != 404:
            raise e
        return None

def create_deployment(namespace: str, body):
    """ Retrieve a deployment from Kubernetes.

    Parameters
    ----------
    namespace : str
        The namespace of a deployment to create.
    body : object
        A deployment to create.

    Returns
    -------
    out : object
        An object containing information about the new deployment.

    Raises
    ------
    kubernetes.client.rest.ApiException
        If an exception occurs during creation.

    """
    k8s_beta = client.ExtensionsV1beta1Api()
    try:
        return k8s_beta.create_namespaced_deployment(namespace, body)
    except kubernetes.client.rest.ApiException as e:
        raise e

def delete_deployment(namespace: str, name: str):
    """ Retrieve a deployment from Kubernetes.

    Parameters
    ----------
    namespace : str
        The namespace of a deployment to remove.
    name : str
        The name of a deployment to remove.

    Returns
    -------
    out : object
        An object containing information about the new deployment.

    Raises
    ------
    kubernetes.client.rest.ApiException
        If an exception occurs during creation.

    """
    k8s_beta = client.ExtensionsV1beta1Api()
    body = kubernetes.client.V1DeleteOptions()
    try:
        return k8s_beta.delete_namespaced_deployment(name, namespace, body)
    except kubernetes.client.rest.ApiException as e:
        raise e

def patch_deployment(namespace: str, name: str, body: dict):
    """ Retrieve a deployment from Kubernetes.

    Parameters
    ----------
    namespace : str
        The namespace of a deployment to remove.
    name : str
        The name of a deployment to remove.
    body : dict
        Attributes to patch.

    Returns
    -------
    out : object
        An object containing information about the patched deployment.

    Raises
    ------
    kubernetes.client.rest.ApiException
        If an exception occurs during creation.

    """
    k8s_beta = client.ExtensionsV1beta1Api()
    try:
        return k8s_beta.patch_namespaced_deployment(name, namespace, body)
    except kubernetes.client.rest.ApiException as e:
        raise e

def clone_deployment(namespace: str, name: str):
    """ Copy a deployment 

    Parameters
    ----------
    namespace : str
        The namespace of a deployment to copy.
    name : str
        The name of a deployment to copy.

    Returns
    -------
    out : (str, str)
        A tuple containing the name of the newly-created deployment
        and a randomized name for the app it contains.

    """
    dep = read_deployment(namespace, name)
    dep.metadata.resource_version = None
    dep.metadata.generate_name = name + '-'
    dep.metadata.name = None

    # now create a randomized app name for this deployment
    temp_app_name = base64.b16encode(os.urandom(32))
    dep.metadata.labels['app'] = temp_app_name
    dep.spec.selector.match_labels['app'] = temp_app_name
    dep.spec.template.metadata.labels['app'] = temp_app_name

    deployment = create_deployment(namespace, dep)
    return deployment.metadata.name, temp_app_name


K8S_NAMESPACE = 'argocd'
SERVICE_NAME = 'argocd-server'
DEPLOYMENT_NAME = 'argocd-server'

service_patch_body = lambda name: {'spec': {'selector': {'app': name}}} 

app_service = read_service(K8S_NAMESPACE, SERVICE_NAME)
print(app_service)

cloned_deployment_name, temp_app_name = clone_deployment(K8S_NAMESPACE, DEPLOYMENT_NAME)

# TODO... wait till ready

# patch app service
out = patch_service(K8S_NAMESPACE, SERVICE_NAME, service_patch_body(temp_app_name))
print('out = ' + str(out))

# wait...

# now update existing deployment
new_body = {}
out = patch_deployment(K8S_NAMESPACE, DEPLOYMENT_NAME, new_body)

# wait...

# now switch back service to original deployment
out = patch_service(K8S_NAMESPACE, SERVICE_NAME, service_patch_body(DEPLOYMENT_NAME))

# now delete new deployment
print('now deleting: ' + cloned_deployment_name)
delete_deployment(K8S_NAMESPACE, cloned_deployment_name)


# KUBECTL_COMMAND=kubectl

# DEPLOYMENT_NAME=$1
# SERVICE_NAME=$2

# SERVICE_JSON=`${KUBECTL_COMMAND} get -o json services/${SERVICE_NAME}`

# copy_deployment() {
# 	DEPLOYMENT_NAME=$1
# 	TIMESTAMP=`date '+%s'`

# 	${KUBECTL_COMMAND} get -o json "deployments/${DEPLOYMENT_NAME}" | jq ".spec.selector.matchLabels.app=.metadata.labels.app=metadata.name=template.metadata.labels.app"

# 	# return the new deployment name
# 	echo "${DEPLOYMENT_NAME}-${TIMESTAMP}"
# }

# NEW_DEPLOYMENT_NAME=copy_deployment ${DEPLOYMENT_NAME}

# # change TEMP deployment spec.selector.matchLabels.app to have suffix
# cat yamldep.json | jq ".spec.selector.matchLabels.app+=\"${TIMESTAMP}\"" | jq '.spec.selector.matchLabels.app'

# # deploy...
# ${KUBECTL_COMMAND} create -f tmpnewyaml.yaml

# # change ORIGINAL service spec.selector.app to be selector with suffix from deployment
# ${KUBECTL_COMMAND} patch -f tmpnewyaml.yaml

# # upgrade ORIGINAL deployment...


# # change ORIGINAL service spec.selector.app to be original selector, without suffix from TEMP deployment
# ${KUBECTL_COMMAND} create -f tmpnewyaml.yaml
