import os
import subprocess
import argparse
from pathlib import Path
from google import genai
from google.genai import types

# === CONFIG ===
client = genai.Client(api_key=os.environ["GEMINI_API_KEY"])
DOCS_REPO_URL = os.environ["DOCS_REPO_URL"]
BRANCH_NAME = "doc-update-from-pr"

def get_diff():
    """Get the full diff for the entire PR, not just the latest commit"""
    # First, try to get PR base from environment (set by GitHub Actions)
    pr_base = os.environ.get("PR_BASE", "origin/main")
    pr_number = os.environ.get("PR_NUMBER", "unknown")

    print(f"Getting diff for PR #{pr_number} against base: {pr_base}")

    # Get the merge-base to ensure we capture all PR changes
    merge_base_result = subprocess.run(
        ["git", "merge-base", pr_base, "HEAD"],
        capture_output=True, text=True
    )

    if merge_base_result.returncode == 0:
        # Use merge-base to get all changes in the PR branch
        merge_base = merge_base_result.stdout.strip()
        print(f"Using merge-base: {merge_base[:7]}...{merge_base[-7:]}")

        # Show which files changed in the entire PR
        files_result = subprocess.run(
            ["git", "diff", "--name-only", f"{merge_base}...HEAD"],
            capture_output=True, text=True
        )
        if files_result.returncode == 0:
            changed_files = files_result.stdout.strip().split('\n')
            changed_files = [f for f in changed_files if f.strip()]
            print(f"Files changed in entire PR: {changed_files}")

        result = subprocess.run(
            ["git", "diff", f"{merge_base}...HEAD"],
            capture_output=True, text=True
        )
        diff_method = f"merge-base ({merge_base[:7]}...HEAD)"
    else:
        # Fallback to the original method
        print("Warning: Could not find merge-base, using fallback diff method")
        result = subprocess.run(
            ["git", "diff", f"{pr_base}...HEAD"],
            capture_output=True, text=True
        )
        diff_method = f"direct ({pr_base}...HEAD)"

    diff_content = result.stdout.strip()
    print(f"Diff method: {diff_method}")
    print(f"Diff size: {len(diff_content)} characters")

    return diff_content

def get_commit_info():
    """Get PR information for the documentation PR reference"""
    try:
        # Get PR number from environment if available
        pr_number = os.environ.get("PR_NUMBER")
        print(f"Debug: PR_NUMBER from environment: '{pr_number}'")

        # Get the HEAD commit - this is what GitHub Actions checked out for the PR
        current_commit_result = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True)
        if current_commit_result.returncode != 0:
            return None
        commit_hash = current_commit_result.stdout.strip()

        # Get remote origin URL to construct proper commit links
        remote_url = subprocess.run(["git", "config", "--get", "remote.origin.url"], capture_output=True, text=True)
        if remote_url.returncode != 0:
            return None

        # Convert SSH URL to HTTPS if needed
        repo_url = remote_url.stdout.strip()
        if repo_url.startswith("git@github.com:"):
            repo_url = repo_url.replace("git@github.com:", "https://github.com/").replace(".git", "")
        elif repo_url.endswith(".git"):
            repo_url = repo_url.replace(".git", "")

        # Get commit details
        short_hash = commit_hash[:7]

        # Return PR information if available, otherwise fallback to commit info
        result = {
            'repo_url': repo_url,
            'current_commit': commit_hash,
            'short_hash': short_hash
        }

        # Check if we have a valid PR number (not None, not empty, not "unknown")
        if pr_number and pr_number.strip() and pr_number != "unknown":
            result['pr_number'] = pr_number
            result['pr_url'] = f"{repo_url}/pull/{pr_number}"
            print(f"Debug: Using PR info - PR #{pr_number}")
        else:
            print(f"Debug: No valid PR number, falling back to commit info")

        return result

    except Exception as e:
        print(f"Warning: Could not get commit info: {e}")
        return None

def clone_docs_repo():
    subprocess.run(["git", "clone", DOCS_REPO_URL, "docs_repo"])
    os.chdir("docs_repo")

    # Try to check out the branch if it already exists
    result = subprocess.run(["git", "ls-remote", "--heads", "origin", BRANCH_NAME], capture_output=True, text=True)
    if result.stdout.strip():
        print(f"Reusing existing branch: {BRANCH_NAME}")
        subprocess.run(["git", "fetch", "origin", BRANCH_NAME])
        subprocess.run(["git", "checkout", BRANCH_NAME])
        subprocess.run(["git", "pull", "origin", BRANCH_NAME])
    else:
        print(f"Creating new branch: {BRANCH_NAME}")
        subprocess.run(["git", "checkout", "-b", BRANCH_NAME])


def get_file_previews():
    previews = []
    adoc_files = list(Path(".").rglob("*.adoc"))
    for path in adoc_files:
        try:
            with open(path, encoding="utf-8") as f:
                lines = f.readlines()[:10]  # Get first 10 lines (or fewer if file is short)
                first_lines = "".join(lines)
                previews.append((str(path), first_lines.strip()))
        except Exception as e:
            print(f"Skipping file {path}: {e}")
    return previews

def ask_gemini_for_relevant_files(diff, file_previews):
    context = "\n\n".join(
        [f"File: {fname}\nPreview:\n{preview}" for fname, preview in file_previews]
    )

    prompt = f"""
You are a documentation assistant.

A code change was made in this PR (Git diff):
{diff}

Below is a list of .adoc documentation files and a preview of their content:

{context}

Based on the diff, which files from this list should be updated? Return only the file paths (one per line). No explanations or extra formatting.
"""

    response = client.models.generate_content(
        model="gemini-2.5-flash",
        contents=prompt,
        config=types.GenerateContentConfig(
            thinking_config=types.ThinkingConfig(thinking_budget=0)
        ),
    )

    # Filter out source code files - only keep .adoc documentation files
    suggested_files = [line.strip() for line in response.text.strip().splitlines() if line.strip()]
    filtered_files = [f for f in suggested_files if f.endswith('.adoc')]

    if len(filtered_files) != len(suggested_files):
        skipped = [f for f in suggested_files if not f.endswith('.adoc')]
        print(f"Skipping non-documentation files: {skipped}")

    return filtered_files

def load_full_content(file_path):
    try:
        return Path(file_path).read_text(encoding="utf-8")
    except Exception as e:
        print(f"Failed to read {file_path}: {e}")
        return ""

def ask_gemini_for_updated_content(diff, file_path, current_content):
    prompt = f"""
You are a documentation assistant.

CRITICAL FORMATTING REQUIREMENTS FOR ASCIIDOC FILES:
- NEVER use markdown code fences like ```adoc or ``` anywhere in the file
- AsciiDoc files start directly with content (comments, headers, or text)
- Use ONLY AsciiDoc syntax: ==== for headers, |=== for tables, ---- for code blocks
- Do NOT mix markdown and AsciiDoc syntax
- Maintain proper table structures with matching |=== opening and closing
- Keep all cross-references (xref) intact and properly formatted
- Ensure consistent indentation and spacing

A developer made the following code changes:
{diff}

Here is the full content of the current documentation file `{file_path}`:
--------------------
{current_content}
--------------------

Analyze the diff and check whether **new, important information** is introduced that is not already covered in this file.

- If the file already includes everything important, return exactly: `NO_UPDATE_NEEDED`
- If the file is missing key information, return the **full updated file content**, modifying only what is necessary. in valid AsciiDoc format

VALIDATION CHECKLIST - Before responding, verify:
1. No markdown code fences (```) anywhere in the content
2. All tables have matching |=== opening and closing
3. All section headers use correct ==== syntax
4. All cross-references are properly formatted
5. No broken formatting or incomplete structures

Do not explain or summarize — only return either:
- `NO_UPDATE_NEEDED` (if nothing is missing), or
- The full updated AsciiDoc file content with perfect syntax (NO markdown!)
"""


    response = client.models.generate_content(
        model="gemini-2.5-flash",
        contents=prompt,
        config=types.GenerateContentConfig(
            thinking_config=types.ThinkingConfig(thinking_budget=0)
        ),
    )
    return response.text.strip()

def overwrite_file(file_path, new_content):
    try:
        Path(file_path).write_text(new_content, encoding="utf-8")
        return True
    except Exception as e:
        print(f"Failed to write {file_path}: {e}")
        return False

def push_and_open_pr(modified_files, commit_info=None):
    subprocess.run(["git", "add"] + modified_files)

    # Build commit message with useful links
    commit_msg = "Auto-generated doc updates from code changes"

    if commit_info:
        if 'pr_number' in commit_info:
            commit_msg += f"\n\nPR Link: {commit_info['pr_url']}"
            commit_msg += f"\nLatest commit: {commit_info['short_hash']}"
        else:
            # Fallback to commit reference if no PR info available
            commit_url = f"{commit_info['repo_url']}/commit/{commit_info['current_commit']}"
            commit_msg += f"\n\nCommit Link: {commit_url}"
            commit_msg += f"\nLatest commit: {commit_info['short_hash']}"

    commit_msg += "\n\nAssisted-by: Gemini"

    subprocess.run([
        "git", "commit",
        "-m", commit_msg
    ])
    # Add remote with token auth
    gh_token = os.environ["GH_TOKEN"]
    docs_repo_url = DOCS_REPO_URL.replace("https://", f"https://{gh_token}@")

    subprocess.run(["git", "remote", "set-url", "origin", docs_repo_url])
    subprocess.run(["git", "push", "--set-upstream", "origin", BRANCH_NAME, "--force"])

    # Build PR body (simple, without commit references)
    pr_body = "This PR updates the following documentation files based on code changes:\n\n"
    pr_body += "\n".join([f"- `{f}`" for f in modified_files])
    pr_body += "\n\n*Note: Each commit in this PR contains references to the specific source code commits that triggered the documentation updates.*"

    subprocess.run([
        "gh", "pr", "create",
        "--title", "Auto-Generated Doc Updates from Code PR",
        "--body", pr_body,
        "--base", "main",
        "--head", BRANCH_NAME
    ])

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dry-run", action="store_true", help="Simulate changes without writing files or pushing PR")
    args = parser.parse_args()

    diff = get_diff()
    if not diff:
        print("No changes detected.")
        return
    # Get commit info before switching to docs repo
    commit_info = get_commit_info()
    if commit_info:
        print(f"Source repository: {commit_info['repo_url']}")
        print(f"Latest commit: {commit_info['short_hash']}")

    clone_docs_repo()
    previews = get_file_previews()

    print("Asking Gemini for relevant files...")
    relevant_files = ask_gemini_for_relevant_files(diff, previews)
    if not relevant_files:
        print("Gemini did not suggest any files.")
        return

    print("Files selected by Gemini:", relevant_files)

    modified_files = []
    for file_path in relevant_files:
        current = load_full_content(file_path)
        if not current:
            continue

        print(f"Checking if {file_path} needs an update...")
        updated = ask_gemini_for_updated_content(diff, file_path, current)

        if updated.strip() == "NO_UPDATE_NEEDED":
            print(f"No update needed for {file_path}")
            continue

        if args.dry_run:
            print(f"[Dry Run] Would update {file_path} with:\n{updated}\n")
        else:
            print(f"Updating {file_path}...")
            if overwrite_file(file_path, updated):
                modified_files.append(file_path)

    if modified_files:
        if args.dry_run:
            print("[Dry Run] Would push and open PR for the following files:")
            for f in modified_files:
                print(f"- {f}")

            if commit_info:
                # Show what the commit message would look like
                commit_msg = "Auto-generated doc updates from code changes"

                if 'pr_number' in commit_info:
                    commit_msg += f"\n\nPR Link: {commit_info['pr_url']}"
                    commit_msg += f"\nLatest commit: {commit_info['short_hash']}"
                else:
                    # Fallback to commit reference if no PR info available
                    commit_url = f"{commit_info['repo_url']}/commit/{commit_info['current_commit']}"
                    commit_msg += f"\n\nCommit Link: {commit_url}"
                    commit_msg += f"\nLatest commit: {commit_info['short_hash']}"

                commit_msg += "\n\nAssisted-by: Gemini"

                print(f"\n[Dry Run] Commit message would be:")
                print("=" * 50)
                print(commit_msg)
                print("=" * 50)

                print(f"\n[Dry Run] PR body would be simple (commit reference is in commit message only)")
        else:
            push_and_open_pr(modified_files, commit_info)
    else:
        print("All documentation is already up to date — no PR created.")

if __name__ == "__main__":
    main()