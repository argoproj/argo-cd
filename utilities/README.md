# ArgoCD Management Tools

A comprehensive collection of Python scripts for managing ArgoCD applications at scale across multiple clusters and environments.

[![Python](https://img.shields.io/badge/Python-3.8+-blue.svg)](https://www.python.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![ArgoCD](https://img.shields.io/badge/ArgoCD-Compatible-green.svg)](https://argoproj.github.io/cd/)

## 🎯 Overview

This toolkit provides enterprise-grade utilities for efficient management of ArgoCD applications across multiple clusters and environments. Whether you're managing hundreds of applications or need detailed insights into your GitOps deployments, these tools have you covered.

## ✨ Features

- 📊 **Multi-cluster Statistics** - Comprehensive application health and sync status across all clusters
- 🎨 **Multiple Output Formats** - Table, CSV, JSON, and HTML with color-coded status indicators
- 🔐 **Secure Authentication** - Token-based authentication with ArgoCD servers
- 🌍 **Environment Filtering** - Support for QA, Production, and multi-environment deployments
- 📈 **Real-time Monitoring** - Track sync operations and health status changes
- 🛠️ **Extensible Architecture** - Easy to extend with additional ArgoCD operations

## 📁 Directory Structure

```
argocd/
├── list_app_stats.py       # Main application statistics tool
├── config.json             # Configuration file (create from template)
├── config.json.template    # Configuration template
├── niks.py                 # Additional utilities
└── README.md               # This file
```

## 🚀 Quick Start

### Prerequisites

- Python 3.8+
- Access to ArgoCD server(s)
- Valid ArgoCD credentials

### Installation

```bash
# Install required dependencies
pip install requests argparse json

# Clone or navigate to the argocd scripts directory
cd tooling/scripts/argocd/
```

### Configuration

1. Copy the configuration template:
```bash
cp config.json.template config.json
```

2. Edit `config.json` with your ArgoCD server details:
```json
{
  "clusters": [
    {
      "name": "qa-cluster",
      "env": "qa",
      "argocd_server": "https://argocd-qa.example.com",
      "username": "your-username",
      "password": "your-password"
    },
    {
      "name": "prod-cluster", 
      "env": "prod",
      "argocd_server": "https://argocd-prod.example.com",
      "username": "your-username",
      "password": "your-password"
    }
  ]
}
```

## 📖 Usage

### Application Statistics

Get comprehensive statistics for all your ArgoCD applications:

```bash
# Default table format for all environments
python list_app_stats.py

# CSV format for easy spreadsheet import
python list_app_stats.py --format csv

# JSON format for programmatic processing
python list_app_stats.py --format json

# HTML format with color-coded status indicators
python list_app_stats.py --format html

# Filter by environment
python list_app_stats.py --env qa
python list_app_stats.py --env prod
```

### Output Formats

#### Table Format (Default)
```
Application Statistics:
--------------------------------------------------------------------------------
Destination              |  Syncing  | Sync:Synced | Sync:OutOfSync | Health:Healthy | Health:Degraded |  Total
--------------------------------------------------------------------------------
qa/cluster-1             |    2      |      45     |       3        |       40       |        8        |   50
prod/cluster-2           |    0      |      92     |       1        |       85       |        7        |   93
--------------------------------------------------------------------------------
TOTAL                    |    2      |     137     |       4        |      125       |       15        |  143
```

#### HTML Format
Generates a beautiful HTML table with:
- 🔴 Red highlighting for OutOfSync/Degraded applications
- 🟢 Green highlighting for Synced/Healthy applications  
- 🟡 Yellow highlighting for Progressing applications
- Color-coded columns for easy visual scanning

## 🔧 Script Details

### [`list_app_stats.py`](tooling/scripts/argocd/list_app_stats.py)

The main statistics tool that provides comprehensive insights into your ArgoCD applications.

**Key Functions:**
- [`load_config()`](tooling/scripts/argocd/list_app_stats.py) - Loads cluster configuration
- [`get_auth_token()`](tooling/scripts/argocd/list_app_stats.py) - Handles ArgoCD authentication
- [`list_argocd_applications()`](tooling/scripts/argocd/list_app_stats.py) - Retrieves application data
- [`collect_all_apps()`](tooling/scripts/argocd/list_app_stats.py) - Aggregates data across clusters
- [`format_statistics()`](tooling/scripts/argocd/list_app_stats.py) - Formats output in multiple formats

**Command Line Options:**
- `--format`: Output format (table, csv, json, html)
- `--env`: Environment filter (qa, prod, all)

## 🎨 Output Examples

### Health Status Tracking
Monitor the health of your applications across environments:
- **Healthy**: Applications running normally
- **Progressing**: Applications currently updating
- **Degraded**: Applications with issues requiring attention
- **Missing**: Applications not found or unreachable

### Sync Status Monitoring
Track GitOps synchronization across your fleet:
- **Synced**: Applications in sync with Git repository
- **OutOfSync**: Applications that need updates
- **Syncing**: Applications currently being synchronized

## 🔒 Security Best Practices

- Store credentials securely (consider using environment variables)
- Use service accounts with minimal required permissions
- Regularly rotate authentication tokens
- Keep configuration files out of version control

## 🤝 Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for:
- Bug fixes
- Feature enhancements
- Documentation improvements
- Additional output formats
- New ArgoCD operations

## 📋 Requirements

- Python 3.8+
- `requests` library
- `argparse` (standard library)
- `json` (standard library)
- Valid ArgoCD server access

## 🐛 Troubleshooting

### Common Issues

1. **Authentication Failed**
   - Verify credentials in config.json
   - Check ArgoCD server URL accessibility
   - Ensure user has proper permissions

2. **Connection Errors**
   - Verify network connectivity to ArgoCD servers
   - Check SSL certificate validity
   - Confirm firewall rules allow access

3. **Empty Results**
   - Verify user has access to applications
   - Check environment filter settings
   - Confirm applications exist in specified clusters

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- ArgoCD team for the excellent GitOps platform
- Python community for the robust ecosystem
- Contributors and users of these tools

---

**Need help?** Open an issue or reach out to the team for support with your ArgoCD management needs.