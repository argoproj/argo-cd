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
