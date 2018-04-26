#! /usr/bin/env python3

import base64
import kubernetes
import os
import sys

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
    out : object
        An object representing the newly-created deployment.

    """
    dep = read_deployment(namespace, name)
    dep.metadata.resource_version = None
    dep.metadata.generate_name = name + '-'
    dep.metadata.name = None

    # now create a randomized app name for this deployment
    temp_app_name = base64.b16encode(os.urandom(31)).decode()
    dep.metadata.labels['app'] = temp_app_name
    dep.spec.selector.match_labels['app'] = temp_app_name
    dep.spec.template.metadata.labels['app'] = temp_app_name

    return create_deployment(namespace, dep)

def bluegreen_deploy(namespace: str, service_name: str, deployment_name: str):
    service_patch_body = lambda name: {'spec': {'selector': {'app': name}}} 

    app_service = read_service(namespace, service_name)
    print(app_service)

    cloned_deployment = clone_deployment(namespace, deployment_name)
    cloned_deployment_name = cloned_deployment.metadata.name
    temp_app_name = cloned_deployment.metadata.labels['app']

    # TODO... wait till ready

    # patch app service
    out = patch_service(namespace, service_name, service_patch_body(temp_app_name))
    print('out = ' + str(out))

    # wait...

    # now update existing deployment
    new_body = {}
    out = patch_deployment(namespace, deployment_name, new_body)

    # wait...

    # now switch back service to original deployment
    out = patch_service(namespace, service_name, service_patch_body(deployment_name))

    # now delete new deployment
    print('now deleting: ' + cloned_deployment_name)
    delete_deployment(namespace, cloned_deployment_name)

if __name__ == '__main__':
    try:
        namespace = sys.argv[1]
        service_name = sys.argv[2]
        deployment_name = sys.argv[3]
    except IndexError as e:
        print('USAGE: {} NAMESPACE MANIFEST'.format(sys.argv[0]))
        sys.exit(1)

    bluegreen_deploy(namespace, service_name, deployment_name)
