import concurrent.futures
from typing import List, Dict
import argparse
import requests
import json
from datetime import datetime, timezone
import time


def load_config(config_file='config.json'):
    """Load configuration from JSON file"""
    try:
        with open(config_file, 'r') as f:
            config = json.load(f)
            if not isinstance(config.get('clusters'), list):
                print("Error: Configuration must contain 'clusters' array")
                return None
            return config
    except FileNotFoundError:
        print(f"Error: Configuration file '{config_file}' not found")
        return None
    except json.JSONDecodeError:
        print(f"Error: Invalid JSON in configuration file '{config_file}'")
        return None


def get_auth_token(argocd_server, username, password):
    """Get authentication token from ArgoCD"""
    login_endpoint = f"{argocd_server}/api/v1/session"
    try:
        response = requests.post(
            login_endpoint,
            json={'username': username, 'password': password},
            verify=True
        )
        response.raise_for_status()
        return response.json()['token']
    except requests.exceptions.RequestException as e:
        print(f"Error during authentication: {e}")
        return None


def abort_sync_operation(argocd_server, token, app_name):
    """Abort sync operation for an application"""
    abort_endpoint = f"{argocd_server}/api/v1/applications/{app_name}/operation"
    headers = {
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json'
    }
    try:
        response = requests.delete(
            abort_endpoint,
            headers=headers,
            verify=True
        )
        response.raise_for_status()
        print(f"Successfully aborted sync operation for {app_name}")
        return True
    except requests.exceptions.RequestException as e:
        print(f"Error aborting sync operation for {app_name}: {e}")
        return False


def get_syncing_applications(argocd_server, token):
    """Get applications that are currently syncing"""
    api_endpoint = f"{argocd_server}/api/v1/applications"
    headers = {
        'Authorization': f'Bearer {token}',
        'Accept': 'application/json'
    }
    try:
        response = requests.get(
            api_endpoint,
            headers=headers,
            verify=True
        )
        response.raise_for_status()
        apps = response.json()['items']

        syncing_apps = []
        current_time = datetime.now(timezone.utc)

        for app in apps:
            if ('operationState' in app['status'] and
                    app['status']['operationState'].get('phase') == 'Running'):

                # Get sync start time
                start_time_str = app['status']['operationState'].get('startedAt')
                if start_time_str:
                    start_time = datetime.strptime(
                        start_time_str,
                        "%Y-%m-%dT%H:%M:%SZ"
                    ).replace(tzinfo=timezone.utc)

                    # Calculate duration in hours
                    duration = (current_time - start_time).total_seconds() / 3600

                    if duration >= 1:
                        syncing_apps.append({
                            'name': app['metadata']['name'],
                            'duration': duration
                        })

        return syncing_apps
    except requests.exceptions.RequestException as e:
        print(f"Error accessing Argo CD API: {e}")
        return None

import concurrent.futures
from typing import List, Dict

def process_cluster(cluster: Dict, target_envs: List[str]) -> None:
    """Process a single cluster concurrently"""
    # Skip if environment doesn't match target
    if cluster.get('env', '').lower() not in target_envs:
        print(f"\nSkipping cluster {cluster['name']} (env: {cluster.get('env')})")
        return

    print(f"\nChecking cluster: {cluster['name']} (env: {cluster.get('env')})")
    token = get_auth_token(
        cluster['argocd_server'],
        cluster['username'],
        cluster['password']
    )

    if not token:
        print(f"Failed to authenticate with ArgoCD server: {cluster['name']}")
        return

    stuck_apps = get_syncing_applications(cluster['argocd_server'], token)

    if not stuck_apps:
        print("No applications stuck in syncing state")
        return

    print(f"Found {len(stuck_apps)} applications stuck in syncing state:")
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        # Process applications concurrently
        futures = [
            executor.submit(
                abort_sync_operation,
                cluster['argocd_server'],
                token,
                app['name']
            )
            for app in stuck_apps
        ]
        for app, future in zip(stuck_apps, futures):
            print(f"- {app['name']} (syncing for {app['duration']:.1f} hours)")
            future.result()

def main():
    parser = argparse.ArgumentParser(
        description='Abort ArgoCD applications stuck in syncing state for >2 hours',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Example config.json:
{
  "clusters": [
    {
      "name": "cluster-name",
      "env": "prod",
      "argocd_server": "https://argocd-server-url",
      "username": "username",
      "password": "password"
    }
  ]
}

Usage:
  python abort_syncing_apps.py
  python abort_syncing_apps.py --env prod
  python abort_syncing_apps.py --env qa
  python abort_syncing_apps.py --help'''
    )
    parser.add_argument(
        '--config',
        default='config.json',
        help='Path to configuration file (default: config.json)'
    )
    parser.add_argument(
        '--env',
        choices=['prod', 'qa'],
        help='Target specific environment (prod or qa). If not specified, both environments are processed'
    )
    args = parser.parse_args()

    config = load_config(args.config)
    if not config:
        return

    target_envs = [args.env.lower()] if args.env else ['prod', 'qa']

    # Process clusters concurrently
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        futures = [
            executor.submit(process_cluster, cluster, target_envs)
            for cluster in config['clusters']
        ]
        concurrent.futures.wait(futures)

if __name__ == "__main__":
    main()