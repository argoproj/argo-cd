local SAVED_ANNOTATION = "argocd.argoproj.io/autosync-saved-config"

if obj.spec == nil then
    obj.spec = {}
end
if obj.spec.syncPolicy == nil then
    obj.spec.syncPolicy = {}
end

if obj.spec.syncPolicy.automated ~= nil then
    -- Currently ENABLED: save prune/selfHeal/allowEmpty to annotation, then disable
    local automated = obj.spec.syncPolicy.automated
    if obj.metadata.annotations == nil then
        obj.metadata.annotations = {}
    end
    local parts = {}
    if automated.prune ~= nil then
        table.insert(parts, "prune=" .. tostring(automated.prune))
    end
    if automated.selfHeal ~= nil then
        table.insert(parts, "selfHeal=" .. tostring(automated.selfHeal))
    end
    if automated.allowEmpty ~= nil then
        table.insert(parts, "allowEmpty=" .. tostring(automated.allowEmpty))
    end
    obj.metadata.annotations[SAVED_ANNOTATION] = table.concat(parts, ",")
    obj.spec.syncPolicy.automated = nil
    if next(obj.spec.syncPolicy) == nil then
        obj.spec.syncPolicy = nil
    end
else
    -- Currently DISABLED: restore from annotation if available
    local automated = {}
    if obj.metadata ~= nil and
       obj.metadata.annotations ~= nil and
       obj.metadata.annotations[SAVED_ANNOTATION] ~= nil then
        local saved = obj.metadata.annotations[SAVED_ANNOTATION]
        for pair in string.gmatch(saved, "([^,]+)") do
            local key, value = string.match(pair, "([^=]+)=(.+)")
            if key and value then
                if value == "true" then
                    automated[key] = true
                elseif value == "false" then
                    automated[key] = false
                end
            end
        end
        obj.metadata.annotations[SAVED_ANNOTATION] = nil
    end
    obj.spec.syncPolicy.automated = automated
end

return obj
