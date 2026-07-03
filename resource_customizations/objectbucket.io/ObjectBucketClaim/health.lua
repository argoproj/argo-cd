-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

-- CRD documentation: https://doc.crds.dev/github.com/kube-object-storage/lib-bucket-provisioner/objectbucket.io/ObjectBucketClaim/v1alpha1@kubernetes-v1.14.1
local hs = {
    status = "Progressing",
    message = "Waiting for status to be reported"
}

-- phase status check - https://github.com/kube-object-storage/lib-bucket-provisioner/blob/ffa47d5/pkg/apis/objectbucket.io/v1alpha1/objectbucketclaim_types.go#L58
if obj.status ~= nil then
    if obj.status.phase ~= nil then
        hs.message = "Object bucket claim phase is " .. obj.status.phase
        if obj.status.phase == "Bound" then
            hs.status = "Healthy"
        elseif obj.status.phase == "Failed" then
            hs.status = "Degraded"
        end
    else
        hs.message = "Waiting for phase to be reported"
    end
end

return hs
