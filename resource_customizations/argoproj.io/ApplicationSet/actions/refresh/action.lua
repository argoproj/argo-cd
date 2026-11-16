if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
-- The ApplicationSet controller re-runs generators (bypassing the Git cache) and removes this annotation
obj.metadata.annotations["argocd.argoproj.io/application-set-refresh"] = "true"
return obj
