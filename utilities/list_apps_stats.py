import argparse
import requests
import json
from requests.auth import HTTPBasicAuth
import os


def load_config(config_file='config.json'):
    """Load configuration from JSON file"""
    try:
        with open(config_file, 'r') as f:
            return json.load(f)
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


def list_argocd_applications(argocd_server, token):
    """List all ArgoCD applications using the API"""
    api_endpoint = f"{argocd_server}/api/v1/applications"

    headers = {
        'Accept': 'application/json',
        'Authorization': f'Bearer {token}'
    }

    try:
        response = requests.get(
            api_endpoint,
            headers=headers,
            verify=True
        )
        response.raise_for_status()
        applications = response.json()
        return applications['items']

    except requests.exceptions.RequestException as e:
        print(f"Error accessing Argo CD API: {e}")
        return None


def get_destination_name(server_url):
    """Extract a friendly name from the server URL"""
    name = server_url.replace('https://', '').replace('http://', '')
    name = name.split('.')[0]  # Take first part of the domain
    return name


def get_cluster_name(server_url, argocd_server, token):
    """Get actual cluster name from ArgoCD API"""
    api_endpoint = f"{argocd_server}/api/v1/clusters"
    headers = {
        'Accept': 'application/json',
        'Authorization': f'Bearer {token}'
    }

    try:
        response = requests.get(
            api_endpoint,
            headers=headers,
            verify=True
        )
        response.raise_for_status()
        clusters = response.json()['items']

        # Find matching cluster by server URL
        for cluster in clusters:
            if cluster['server'] == server_url:
                # Use name if available, otherwise fallback to server URL
                return cluster.get('name') or cluster['server']

        # Fallback to server URL if cluster not found
        return server_url.replace('https://', '').replace('http://', '').split('.')[0]

    except requests.exceptions.RequestException as e:
        print(f"Error accessing Argo CD Clusters API: {e}")
        return server_url.replace('https://', '').replace('http://', '').split('.')[0]


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


def collect_all_apps(config):
    """Collect applications from all configured ArgoCD servers"""
    all_apps = []

    for cluster_config in config['clusters']:
        if not all(key in cluster_config for key in ['name', 'env', 'argocd_server', 'username', 'password']):
            print(f"Error: Missing required configuration for cluster {cluster_config.get('name', 'Unknown')}")
            continue

        token = get_auth_token(cluster_config['argocd_server'],
                               cluster_config['username'],
                               cluster_config['password'])

        if not token:
            print(f"Failed to authenticate with ArgoCD server: {cluster_config['name']}")
            continue

        apps = list_argocd_applications(cluster_config['argocd_server'], token)
        if apps:
            # Add environment and ArgoCD cluster name information to each app
            for app in apps:
                app['argocd_env'] = cluster_config['env']
                app['argocd_name'] = cluster_config['name']
                all_apps.append(app)

    return all_apps


def format_statistics(apps, configs, output_format='table'):
    """Format statistics in the specified format (table, csv, json, or html)"""
    stats = {}
    all_sync_statuses = set()
    all_health_statuses = set()

    # Create a mapping of server URLs to cluster names
    cluster_names = {}
    for config in configs['clusters']:
        token = get_auth_token(config['argocd_server'], config['username'], config['password'])
        if token:
            cluster_api_endpoint = f"{config['argocd_server']}/api/v1/clusters"
            headers = {
                'Accept': 'application/json',
                'Authorization': f'Bearer {token}'
            }
            try:
                response = requests.get(cluster_api_endpoint, headers=headers, verify=True)
                response.raise_for_status()
                for cluster in response.json()['items']:
                    key = f"{config['env']}:{cluster['server']}"
                    cluster_names[key] = f"{config['env']}/{cluster.get('name', cluster['server'])}"
            except requests.exceptions.RequestException:
                pass

    for app in apps:
        server_url = app['spec']['destination']['server']
        env = app['argocd_env']
        key = f"{env}:{server_url}"

        # Use environment-prefixed cluster name or fallback to server URL
        cluster = cluster_names.get(key, f"{env}/{server_url.split('/')[-1]}")

        sync_status = app['status']['sync']['status']
        health_status = app['status']['health']['status']
        is_syncing = 'operationState' in app['status'] and app['status']['operationState'].get('phase') == 'Running'

        if cluster not in stats:
            stats[cluster] = {
                'sync': {},
                'health': {},
                'syncing': 0
            }

        stats[cluster]['sync'][sync_status] = stats[cluster]['sync'].get(sync_status, 0) + 1
        stats[cluster]['health'][health_status] = stats[cluster]['health'].get(health_status, 0) + 1
        if is_syncing:
            stats[cluster]['syncing'] += 1

        all_sync_statuses.add(sync_status)
        all_health_statuses.add(health_status)

    # Rest of the formatting remains the same
    if output_format == 'json':
        return _format_json(stats)
    elif output_format == 'csv':
        return _format_csv(stats, all_sync_statuses, all_health_statuses)
    elif output_format == 'html':
        return _format_html(stats, all_sync_statuses, all_health_statuses)
    else:  # default to table
        return _format_table(stats, all_sync_statuses, all_health_statuses)


def _format_csv(stats, sync_statuses, health_statuses):
    """Format statistics as CSV"""
    output = []
    headers = ["Destination", "Syncing"]
    headers.extend([f"Sync:{status}" for status in sorted(sync_statuses)])
    headers.extend([f"Health:{status}" for status in sorted(health_statuses)])
    headers.append("Total")
    output.append(",".join(headers))

    for destination in sorted(stats.keys()):
        row = [destination, str(stats[destination]['syncing'])]
        destination_total = stats[destination]['syncing']

        for status in sorted(sync_statuses):
            count = stats[destination]['sync'].get(status, 0)
            row.append(str(count))
            destination_total += count

        for status in sorted(health_statuses):
            count = stats[destination]['health'].get(status, 0)
            row.append(str(count))

        row.append(str(destination_total))
        output.append(",".join(row))

    return "\n".join(output)


def _format_table(stats, sync_statuses, health_statuses):
    """Format statistics as ASCII table"""
    # Calculate dynamic width based on content
    dest_width = max(20, max(len(dest) for dest in stats.keys()) + 2)
    sync_width = 10
    status_width = 12
    total_width = 8

    # Calculate total table width
    table_width = (dest_width + 3 +  # Destination + separator
                   sync_width + 3 +  # Syncing + separator
                   (status_width + 3) * len(sync_statuses) +  # Sync statuses
                   (status_width + 3) * len(health_statuses) +  # Health statuses
                   total_width)  # Total column

    output = ["\nApplication Statistics:", "-" * table_width]

    # Header
    header = "Destination".ljust(dest_width) + " | "
    header += "Syncing".center(sync_width) + " | "
    for status in sorted(sync_statuses):
        header += f"Sync:{status}".center(status_width) + " | "
    for status in sorted(health_statuses):
        header += f"Health:{status}".center(status_width) + " | "
    header += "Total".center(total_width)
    output.append(header)
    output.append("-" * table_width)

    grand_totals = {'syncing': 0, 'sync': {}, 'health': {}}

    for destination in sorted(stats.keys()):
        row = destination[:dest_width - 1].ljust(dest_width) + " | "
        destination_total = stats[destination]['syncing']
        row += str(stats[destination]['syncing']).center(sync_width) + " | "
        grand_totals['syncing'] += stats[destination]['syncing']

        for status in sorted(sync_statuses):
            count = stats[destination]['sync'].get(status, 0)
            row += str(count).center(status_width) + " | "
            grand_totals['sync'][status] = grand_totals['sync'].get(status, 0) + count
            destination_total += count

        for status in sorted(health_statuses):
            count = stats[destination]['health'].get(status, 0)
            row += str(count).center(status_width) + " | "
            grand_totals['health'][status] = grand_totals['health'].get(status, 0) + count

        row += str(destination_total).center(total_width)
        output.append(row)

    # Footer with totals
    output.append("-" * table_width)
    total_row = "TOTAL".ljust(dest_width) + " | "
    grand_total = grand_totals['syncing']
    total_row += str(grand_totals['syncing']).center(sync_width) + " | "

    for status in sorted(sync_statuses):
        total_row += str(grand_totals['sync'].get(status, 0)).center(status_width) + " | "
        grand_total += grand_totals['sync'].get(status, 0)

    for status in sorted(health_statuses):
        total_row += str(grand_totals['health'].get(status, 0)).center(status_width) + " | "

    total_row += str(grand_total).center(total_width)
    output.append(total_row)

    return "\n".join(output)


def _format_html(stats, sync_statuses, health_statuses):
    """Format statistics as HTML table with color coding"""
    html = ['<table border="1" style="border-collapse: collapse; width: 100%;">']

    # CSS styles with color coding
    styles = """
    <style>
        table { margin: 20px 0; }
        th, td {
            padding: 8px;
            text-align: center;
            border: 1px solid black;
        }
        th { background-color: #f2f2f2; }
        tr:last-child { font-weight: bold; background-color: #f2f2f2; }

        /* Column-specific colors */
        td:first-child, th:first-child { background-color: #E0FFFF; }  /* Light Cyan for Destination */
        td:nth-child(2), th:nth-child(2) { background-color: #FFFFC0; }  /* Light Yellow for Syncing */

        /* Sync status columns */
        td[data-status="Sync:OutOfSync"],
        th[data-status="Sync:OutOfSync"] {
            background-color: #FFE0E0;  /* Light Red */
        }

        td[data-status="Sync:Synced"],
        th[data-status="Sync:Synced"] {
            background-color: #E0FFE0;  /* Light Green */
        }

        /* Health columns */
        td[data-status="Health:Degraded"],
        th[data-status="Health:Degraded"],
        td[data-status="Health:Missing"],
        th[data-status="Health:Missing"] {
            background-color: #FFE0E0;  /* Light Red */
        }

        td[data-status="Health:Progressing"],
        th[data-status="Health:Progressing"] {
            background-color: #FFFFC0;  /* Light Yellow */
        }

        td[data-status="Health:Healthy"],
        th[data-status="Health:Healthy"] {
            background-color: #E0FFE0;  /* Light Green */
        }

        /* Total column */
        td:last-child, th:last-child {
            background-color: #FFE4B5;  /* Light Orange */
        }
    </style>
    """

    html.insert(0, styles)

    # Header
    html.append('<tr>')
    html.append('<th>Destination</th>')
    html.append('<th>Syncing</th>')
    for status in sorted(sync_statuses):
        html.append(f'<th data-status="Sync:{status}">Sync:{status}</th>')
    for status in sorted(health_statuses):
        html.append(f'<th data-status="Health:{status}">Health:{status}</th>')
    html.append('<th>Total</th>')
    html.append('</tr>')

    # Initialize grand totals
    grand_totals = {'syncing': 0, 'sync': {}, 'health': {}}

    # Data rows
    for destination in sorted(stats.keys()):
        html.append('<tr>')
        html.append(f'<td>{destination}</td>')
        html.append(f'<td>{stats[destination]["syncing"]}</td>')
        destination_total = stats[destination]['syncing']
        grand_totals['syncing'] += stats[destination]['syncing']

        for status in sorted(sync_statuses):
            count = stats[destination]['sync'].get(status, 0)
            html.append(f'<td data-status="Sync:{status}">{count}</td>')
            grand_totals['sync'][status] = grand_totals['sync'].get(status, 0) + count
            destination_total += count

        for status in sorted(health_statuses):
            count = stats[destination]['health'].get(status, 0)
            html.append(f'<td data-status="Health:{status}">{count}</td>')
            grand_totals['health'][status] = grand_totals['health'].get(status, 0) + count

        html.append(f'<td>{destination_total}</td>')
        html.append('</tr>')

    # Total row
    html.append('<tr>')
    html.append('<td>TOTAL</td>')
    grand_total = grand_totals['syncing']
    html.append(f'<td>{grand_totals["syncing"]}</td>')

    for status in sorted(sync_statuses):
        count = grand_totals['sync'].get(status, 0)
        html.append(f'<td data-status="Sync:{status}">{count}</td>')
        grand_total += count

    for status in sorted(health_statuses):
        count = grand_totals['health'].get(status, 0)
        html.append(f'<td data-status="Health:{status}">{count}</td>')

    html.append(f'<td>{grand_total}</td>')
    html.append('</tr>')

    html.append('</table>')
    return "\n".join(html)


def _format_json(stats):
    """Format statistics as JSON"""
    return json.dumps(stats, indent=2)


def main():
    parser = argparse.ArgumentParser(description='List ArgoCD applications and show statistics')
    parser.add_argument('--format',
                        choices=['table', 'csv', 'json', 'html'],
                        default='table',
                        help='Output format for statistics (default: table)')
    parser.add_argument('--env',
                        choices=['qa', 'prod', 'all'],
                        default='all',
                        help='Environment to show statistics for (default: all)')
    args = parser.parse_args()

    config = load_config()
    if not config:
        return

    # Collect apps from all configured ArgoCD servers
    all_apps = collect_all_apps(config)

    if not all_apps:
        print("No applications found")
        return

    # Filter apps by environment if specified
    if args.env != 'all':
        all_apps = [app for app in all_apps if app['argocd_env'] == args.env]
        # print("\nArgoCD Applications:")
        # print("-------------------")
        # for app in apps:
        #     print(f"Name: {app['metadata']['name']}")
        #     print(f"Project: {app['spec']['project']}")
        #     print(f"Destination: {app['spec']['destination']['server']}")
        #     print(f"Status: {app['status']['sync']['status']}")
        #     print("-------------------")

        # Print statistics in specified format only
        print(format_statistics(all_apps, config, args.format))


if __name__ == "__main__":
    main()