-- Waiting for status info => Progressing
if obj.status == nil or obj.status.PostgresClusterStatus == nil then
  return {
    status = "Progressing",
    message = "Waiting for postgres cluster status...",
  }
end

-- Running => Healthy
if obj.status.PostgresClusterStatus == "Running" then
  return {
    status = "Healthy",
    message = obj.status.PostgresClusterStatus,
  }
end

-- Creating/Updating => Progressing
if obj.status.PostgresClusterStatus == "Creating" or obj.status.PostgresClusterStatus == "Updating" then
  return {
    status = "Progressing",
    message = obj.status.PostgresClusterStatus,
  }
end

-- CreateFailed/UpdateFailed/SyncFailed/Invalid/etc => Degraded
-- See https://github.com/zalando/postgres-operator/blob/0745ce7c/pkg/apis/acid.zalan.do/v1/const.go#L4-L13
return {
  status = "Degraded",
  message = obj.status.PostgresClusterStatus,
}