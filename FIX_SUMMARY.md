# Fix for Argo CD Issue #24065: ARGOCD_OPTS does not accept --header parameter multiple times

## Problem Description

The Argo CD CLI accepts the `--header` option multiple times when passed directly on the command line, but when using the `ARGOCD_OPTS` environment variable, only a single `--header` is recognized. Multiple `--header` options get overwritten, leaving only the last one.

### Example of the Issue

```bash
# This works correctly (direct CLI usage)
argocd app list --header "CF-Access-Client-Id: foo" --header "CF-Access-Client-Secret: bar"

# This doesn't work (ARGOCD_OPTS usage)
export ARGOCD_OPTS='--header "CF-Access-Client-Id: foo" --header "CF-Access-Client-Secret: bar"'
argocd app list  # Only the last header is sent
```

## Root Cause

The issue was in the `LoadFlags()` function in `util/config/env.go`. The function was designed to store only the last value for each flag key, which works fine for single-value flags but breaks multi-value flags like `--header`.

### Original Code Problem

```go
// This overwrites previous values
flags[key] = opt
```

## Solution

Modified the `LoadFlags()` function to properly handle multi-value flags by:

1. **Adding a new data structure**: `multiFlags map[string][]string` to store multiple values for flags that support them
2. **Identifying multi-value flags**: Added `isMultiValueFlag()` function to identify flags that can have multiple values
3. **Collecting all values**: For multi-value flags, collect all occurrences instead of overwriting
4. **Updating the getter**: Modified `GetStringSliceFlag()` to check for multiple values first

### Key Changes

1. **New data structure for multi-value flags**:
   ```go
   var multiFlags map[string][]string
   ```

2. **Flag identification**:
   ```go
   func isMultiValueFlag(key string) bool {
       multiValueFlags := []string{"header"}
       for _, flag := range multiValueFlags {
           if key == flag {
               return true
           }
       }
       return false
   }
   ```

3. **Value collection logic**:
   ```go
   if isMultiValueFlag(key) {
       if _, exists := multiFlags[key]; !exists {
           multiFlags[key] = []string{}
       }
       multiFlags[key] = append(multiFlags[key], opt)
   } else {
       flags[key] = opt
   }
   ```

4. **Updated getter**:
   ```go
   func GetStringSliceFlag(key string, fallback []string) []string {
       // First check if we have multiple values for this flag
       if multiValues, ok := multiFlags[key]; ok {
           return multiValues
       }
       
       // Fall back to the original single-value behavior
       // ... existing code ...
   }
   ```

## Testing

Added comprehensive tests to verify the fix:

1. **Unit tests** in `util/config/env_test.go`:
   - `TestMultipleHeaderFlags`: Tests multiple `--header` flags
   - `TestMultipleHeaderFlagsWithOtherFlags`: Tests headers mixed with other flags
   - `TestMultipleHeaderFlagsMixedWithCommaSeparated`: Tests mixed usage

2. **Integration test**: Verified end-to-end functionality

### Test Results

All tests pass, confirming that:
- Multiple `--header` flags in `ARGOCD_OPTS` work correctly
- Other flags continue to work as expected
- Existing comma-separated header behavior is preserved
- No regression in existing functionality

## Backward Compatibility

This fix is fully backward compatible:
- Existing single-value flag behavior is unchanged
- Comma-separated headers in a single `--header` option still work
- All other flags continue to work as before

## Usage Examples

### Now Working Correctly

```bash
# Multiple headers in ARGOCD_OPTS
export ARGOCD_OPTS='--header "CF-Access-Client-Id: foo" --header "CF-Access-Client-Secret: bar"'
argocd app list

# Mixed with other flags
export ARGOCD_OPTS='--server localhost:8080 --header "CF-Access-Client-Id: foo" --header "CF-Access-Client-Secret: bar" --insecure'
argocd app list

# Still works (existing behavior)
export ARGOCD_OPTS='--header "CF-Access-Client-Id: foo,CF-Access-Client-Secret: bar"'
argocd app list
```

## Files Modified

1. `util/config/env.go` - Main fix implementation
2. `util/config/env_test.go` - Added comprehensive tests

## Impact

This fix resolves the inconsistency between direct CLI usage and `ARGOCD_OPTS` usage, making the behavior identical in both cases. Users can now use multiple `--header` options in `ARGOCD_OPTS` just like they would on the command line.