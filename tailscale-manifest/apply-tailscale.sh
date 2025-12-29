#!/bin/bash

# Script to apply Tailscale manifests to a Kubernetes cluster
# Usage: ./apply-tailscale.sh --authkey <TS_AUTHKEY> --context <CLUSTER_CONTEXT>

set -e  # Exit on any error

# Default values
AUTH_KEY=""
LOGIN_SERVER=""
CLUSTER_CONTEXT=""
USE_CONTEXT_FLAG=""
VERBOSE=false

# Function to display usage
usage() {
    echo "Usage: $0 --authkey <TS_AUTHKEY> [--login-server <LOGIN_SERVER_URL>] [--context <CLUSTER_CONTEXT>] [-v]"
    echo "  --authkey: Tailscale auth key (required)"
    echo "  --login-server: Tailscale login server URL (optional, uses default if not specified)"
    echo "  --context: Kubernetes cluster context (optional, uses current context if not specified)"
    echo "  -v: Enable verbose output for debugging"
    echo ""
    echo "Example: $0 --authkey tskey-1234567890 --login-server https://my-login-server.example.com --context my-cluster-context -v"
    echo "Example without context (uses current context): $0 --authkey tskey-1234567890 -v"
    exit 1
}

# Function for verbose logging
verbose_log() {
    if [[ "$VERBOSE" == true ]]; then
        echo "[DEBUG] $1" >&2
    fi
}

# Function to validate required arguments
validate_arguments() {
    if [[ -z "$AUTH_KEY" ]]; then
        echo "Error: --authkey is required."
        usage
    fi

    verbose_log "Validated required argument: AUTH_KEY is set"
}

# Function to validate kubectl availability
validate_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        echo "Error: kubectl is not installed or not in PATH."
        exit 1
    fi

    verbose_log "kubectl is available: $(kubectl version --client --short 2>/dev/null || kubectl version --client 2>/dev/null | head -1 || echo 'kubectl available')"
}

# Function to handle Kubernetes context
handle_kubernetes_context() {
    if [[ -n "$CLUSTER_CONTEXT" ]]; then
        # Check if the specified context exists
        if ! kubectl config get-contexts "$CLUSTER_CONTEXT" &> /dev/null; then
            echo "Error: Kubernetes context '$CLUSTER_CONTEXT' does not exist."
            echo "Available contexts:"
            kubectl config get-contexts
            exit 1
        fi

        verbose_log "Kubernetes context '$CLUSTER_CONTEXT' exists and is accessible"
        USE_CONTEXT_FLAG="--context $CLUSTER_CONTEXT"
        echo "Using Kubernetes context: $CLUSTER_CONTEXT"
    else
        # Get the current context
        CURRENT_CONTEXT=$(kubectl config current-context 2>/dev/null)

        # Check if current context is empty or invalid
        if [[ -z "$CURRENT_CONTEXT" ]]; then
            echo "Error: No current kubectl context is set. Please set a context using 'kubectl config use-context' or specify a context with --context."
            exit 1
        fi

        verbose_log "No context specified, using current context: $CURRENT_CONTEXT"
        echo "Using current Kubernetes context: $CURRENT_CONTEXT"
    fi
}

# Function to update the auth secret file
update_auth_secret() {
    echo "Auth key will be configured (not shown for security)"

    # Create a temporary file with the updated auth key
    TEMP_SECRET_FILE=$(mktemp)
    trap 'rm -f "$TEMP_SECRET_FILE"' EXIT

    verbose_log "Created temporary file: $TEMP_SECRET_FILE"

    # Copy the original secret file and replace the auth key
    verbose_log "Updating auth key in tailscale-auth-secret.yaml"
    sed "s|TS_AUTHKEY: tskey-xxxxxxxxxx|TS_AUTHKEY: $AUTH_KEY|" tailscale-auth-secret.yaml > "$TEMP_SECRET_FILE"

    verbose_log "Updated auth key in temporary file (content not shown for security)"

    echo "Applying Tailscale auth secret..."
    verbose_log "Running: kubectl apply -f $TEMP_SECRET_FILE $USE_CONTEXT_FLAG"
    kubectl apply -f "$TEMP_SECRET_FILE" $USE_CONTEXT_FLAG
}

# Function to update the userspace proxy file with TS_EXTRA_ARGS if login server is provided
update_userspace_proxy() {
    # Create TS_EXTRA_ARGS value if login server is provided
    TS_EXTRA_ARGS_VALUE=""
    if [[ -n "$LOGIN_SERVER" ]]; then
        TS_EXTRA_ARGS_VALUE="--login-server=$LOGIN_SERVER"
        verbose_log "Setting TS_EXTRA_ARGS to: $TS_EXTRA_ARGS_VALUE"
    fi

    # Create a temporary file with the updated userspace proxy
    TEMP_PROXY_FILE=$(mktemp)
    trap 'rm -f "$TEMP_PROXY_FILE"' RETURN

    verbose_log "Created temporary file: $TEMP_PROXY_FILE"

    # Copy the original userspace proxy file
    cp tailscale-userspace-proxy.yaml "$TEMP_PROXY_FILE"

    # Replace the placeholder with the actual value or empty if not provided
    if [[ -n "$TS_EXTRA_ARGS_VALUE" ]]; then
        verbose_log "Replacing TS_EXTRA_ARGS placeholder with: $TS_EXTRA_ARGS_VALUE"
        sed -i "s|value: \"TS_EXTRA_ARGS_PLACEHOLDER\"|value: \"$TS_EXTRA_ARGS_VALUE\"|" "$TEMP_PROXY_FILE"
    else
        verbose_log "Removing TS_EXTRA_ARGS placeholder (using default empty value)"
        # Remove the entire TS_EXTRA_ARGS environment variable entry
        sed -i '/- name: TS_EXTRA_ARGS/,/value: "TS_EXTRA_ARGS_PLACEHOLDER"/d' "$TEMP_PROXY_FILE"
    fi

    verbose_log "Applying Tailscale userspace proxy..."
    verbose_log "Running: kubectl apply -f $TEMP_PROXY_FILE $USE_CONTEXT_FLAG"
    kubectl apply -f "$TEMP_PROXY_FILE" $USE_CONTEXT_FLAG
}

# Function to apply the userspace proxy
apply_userspace_proxy() {
    echo "Applying Tailscale userspace proxy..."
    update_userspace_proxy
}

# Function to display completion message
display_completion_message() {
    # Determine which context name to display
    if [[ -n "$CLUSTER_CONTEXT" ]]; then
        CONTEXT_NAME="$CLUSTER_CONTEXT"
    else
        CONTEXT_NAME=$(kubectl config current-context 2>/dev/null)
        # This should not happen since we already validated it above, but just to be safe
        if [[ -z "$CONTEXT_NAME" ]]; then
            echo "Error: Current kubectl context is not set properly."
            exit 1
        fi
    fi

    echo "Tailscale manifests applied successfully to context: $CONTEXT_NAME"
    echo "Deployment may take a moment to become ready. You can check status with:"
    echo "  kubectl $USE_CONTEXT_FLAG get pods"
    verbose_log "Script execution completed successfully"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --authkey)
            AUTH_KEY="$2"
            shift 2
            ;;
        --login-server)
            LOGIN_SERVER="$2"
            shift 2
            ;;
        --context)
            CLUSTER_CONTEXT="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown argument: $1"
            usage
            ;;
    esac
done

# Main execution flow
validate_arguments
validate_kubectl
handle_kubernetes_context
update_auth_secret
apply_userspace_proxy
display_completion_message
