#!/bin/bash

# CSV Import Script for FlexPrice
# This script imports CSV data into the FlexPrice database

set -e

# Default values
TENANT_ID=""
ENVIRONMENT_ID=""
CREATED_BY="csv-import"
UPDATED_BY="csv-import"
DRY_RUN_MODE="false"
ENTITY_TYPE=""
CSV_FILE_PATH=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -t, --tenant-id ID        Tenant ID (required)"
    echo "  -e, --environment-id ID   Environment ID (required)"
    echo "  -c, --created-by USER     Created by user (default: csv-import)"
    echo "  -u, --updated-by USER     Updated by user (default: csv-import)"
    echo "  -d, --dry-run             Run in dry-run mode (default: false)"
    echo "  -f, --file PATH           CSV file path (required)"
    echo "  -s, --entity-type TYPE    Entity type (required)"
    echo "  -h, --help                Show this help message"
    echo ""
    echo "Supported entity types:"
    echo "  plan, price, meter, feature, addon, customer"
    echo ""
    echo "Examples:"
    echo "  $0 -t tenant123 -e env456 -f plans.csv -s plan"
    echo "  $0 -t tenant123 -e env456 -f prices.csv -s price --dry-run"
    echo "  $0 -t tenant123 -e env456 -f customers.csv -s customer -c admin -u admin"
}

# Function to validate entity type
validate_entity_type() {
    local entity_type=$1
    case $entity_type in
        plan|price|meter|feature|addon|customer)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Function to check if file exists
check_file_exists() {
    local file_path=$1
    if [ ! -f "$file_path" ]; then
        print_error "CSV file does not exist: $file_path"
        exit 1
    fi
}

# Function to validate CSV file format
validate_csv_file() {
    local file_path=$1
    local entity_type=$2
    
    print_info "Validating CSV file: $file_path"
    
    # Check if file is readable
    if [ ! -r "$file_path" ]; then
        print_error "Cannot read CSV file: $file_path"
        exit 1
    fi
    
    # Check if file has content
    if [ ! -s "$file_path" ]; then
        print_error "CSV file is empty: $file_path"
        exit 1
    fi
    
    # Check if file has at least 2 lines (header + data)
    local line_count=$(wc -l < "$file_path")
    if [ "$line_count" -lt 2 ]; then
        print_error "CSV file must have at least 2 lines (header + data): $file_path"
        exit 1
    fi
    
    print_success "CSV file validation passed"
}

# Function to show CSV template
show_csv_template() {
    local entity_type=$1
    
    print_info "CSV Template for entity type: $entity_type"
    echo ""
    
    case $entity_type in
        plan)
            echo "name,lookup_key,description,display_order,metadata"
            echo "Basic Plan,basic-plan,Basic subscription plan,1,{\"category\":\"basic\"}"
            echo "Pro Plan,pro-plan,Professional subscription plan,2,{\"category\":\"professional\"}"
            ;;
        price)
            echo "amount,currency,type,display_amount,price_unit_type,billing_period,billing_period_count,billing_model,metadata"
            echo "29.99,usd,RECURRING,\$29.99,FIAT,MONTHLY,1,PER_UNIT,{\"tier\":\"basic\"}"
            echo "99.99,usd,RECURRING,\$99.99,FIAT,MONTHLY,1,PER_UNIT,{\"tier\":\"pro\"}"
            ;;
        meter)
            echo "name,event_name,aggregation_type,aggregation_field,aggregation_multiplier,bucket_size,filters,reset_usage"
            echo "API Calls,api_call,COUNT,,,,\"[{\"key\":\"endpoint\",\"values\":[\"api/v1/users\",\"api/v1/orders\"]}]\",BILLING_PERIOD"
            echo "Storage Usage,storage_usage,SUM,bytes,1,,,BILLING_PERIOD"
            ;;
        feature)
            echo "name,lookup_key,description,type,meter_id,unit_singular,unit_plural,metadata"
            echo "API Calls,api-calls,Number of API calls,USAGE,meter123,call,calls,{\"category\":\"api\"}"
            echo "Storage,storage,Storage usage,USAGE,meter456,GB,GB,{\"category\":\"storage\"}"
            ;;
        addon)
            echo "name,lookup_key,description,type,metadata"
            echo "Priority Support,priority-support,24/7 priority support,SUPPORT,{\"category\":\"support\"}"
            echo "Advanced Analytics,advanced-analytics,Advanced analytics dashboard,FEATURE,{\"category\":\"analytics\"}"
            ;;
        customer)
            echo "external_id,name,email,address_line1,address_line2,address_city,address_state,address_postal_code,address_country,metadata"
            echo "cust_001,John Doe,john@example.com,123 Main St,Apt 4B,New York,NY,10001,US,{\"source\":\"website\"}"
            echo "cust_002,Jane Smith,jane@example.com,456 Oak Ave,,Los Angeles,CA,90210,US,{\"source\":\"referral\"}"
            ;;
    esac
    echo ""
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--tenant-id)
            TENANT_ID="$2"
            shift 2
            ;;
        -e|--environment-id)
            ENVIRONMENT_ID="$2"
            shift 2
            ;;
        -c|--created-by)
            CREATED_BY="$2"
            shift 2
            ;;
        -u|--updated-by)
            UPDATED_BY="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN_MODE="true"
            shift
            ;;
        -f|--file)
            CSV_FILE_PATH="$2"
            shift 2
            ;;
        -s|--entity-type)
            ENTITY_TYPE="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        --template)
            if [ -n "$2" ]; then
                show_csv_template "$2"
                exit 0
            else
                print_error "Entity type required for template"
                exit 1
            fi
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$TENANT_ID" ]; then
    print_error "Tenant ID is required"
    show_usage
    exit 1
fi

if [ -z "$ENVIRONMENT_ID" ]; then
    print_error "Environment ID is required"
    show_usage
    exit 1
fi

if [ -z "$ENTITY_TYPE" ]; then
    print_error "Entity type is required"
    show_usage
    exit 1
fi

if [ -z "$CSV_FILE_PATH" ]; then
    print_error "CSV file path is required"
    show_usage
    exit 1
fi

# Validate entity type
if ! validate_entity_type "$ENTITY_TYPE"; then
    print_error "Invalid entity type: $ENTITY_TYPE"
    print_info "Valid types: plan, price, meter, feature, addon, customer"
    exit 1
fi

# Check if CSV file exists
check_file_exists "$CSV_FILE_PATH"

# Validate CSV file
validate_csv_file "$CSV_FILE_PATH" "$ENTITY_TYPE"

# Print configuration
print_info "CSV Import Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  Created By: $CREATED_BY"
echo "  Updated By: $UPDATED_BY"
echo "  Entity Type: $ENTITY_TYPE"
echo "  CSV File: $CSV_FILE_PATH"
echo "  Dry Run: $DRY_RUN_MODE"
echo ""

# Confirm before proceeding
if [ "$DRY_RUN_MODE" = "false" ]; then
    read -p "Are you sure you want to import data into the database? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Import cancelled"
        exit 0
    fi
fi

# Set environment variables
export TENANT_ID
export ENVIRONMENT_ID
export CREATED_BY
export UPDATED_BY
export DRY_RUN_MODE
export ENTITY_TYPE
export CSV_FILE_PATH

# Run the import
print_info "Starting CSV import..."

# Change to the scripts directory
cd "$(dirname "$0")"

# Run the Go import function
go run -tags tools ./internal/csv_import.go

if [ $? -eq 0 ]; then
    print_success "CSV import completed successfully!"
else
    print_error "CSV import failed!"
    exit 1
fi

