// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    accountLoginCompletedNotification, err := UnmarshalAccountLoginCompletedNotification(bytes)
//    bytes, err = accountLoginCompletedNotification.Marshal()
//
//    accountRateLimitsUpdatedNotification, err := UnmarshalAccountRateLimitsUpdatedNotification(bytes)
//    bytes, err = accountRateLimitsUpdatedNotification.Marshal()
//
//    accountUpdatedNotification, err := UnmarshalAccountUpdatedNotification(bytes)
//    bytes, err = accountUpdatedNotification.Marshal()
//
//    agentMessageDeltaNotification, err := UnmarshalAgentMessageDeltaNotification(bytes)
//    bytes, err = agentMessageDeltaNotification.Marshal()
//
//    appListUpdatedNotification, err := UnmarshalAppListUpdatedNotification(bytes)
//    bytes, err = appListUpdatedNotification.Marshal()
//
//    appsListParams, err := UnmarshalAppsListParams(bytes)
//    bytes, err = appsListParams.Marshal()
//
//    appsListResponse, err := UnmarshalAppsListResponse(bytes)
//    bytes, err = appsListResponse.Marshal()
//
//    cancelLoginAccountParams, err := UnmarshalCancelLoginAccountParams(bytes)
//    bytes, err = cancelLoginAccountParams.Marshal()
//
//    cancelLoginAccountResponse, err := UnmarshalCancelLoginAccountResponse(bytes)
//    bytes, err = cancelLoginAccountResponse.Marshal()
//
//    commandExecOutputDeltaNotification, err := UnmarshalCommandExecOutputDeltaNotification(bytes)
//    bytes, err = commandExecOutputDeltaNotification.Marshal()
//
//    commandExecParams, err := UnmarshalCommandExecParams(bytes)
//    bytes, err = commandExecParams.Marshal()
//
//    commandExecResizeParams, err := UnmarshalCommandExecResizeParams(bytes)
//    bytes, err = commandExecResizeParams.Marshal()
//
//    commandExecResizeResponse, err := UnmarshalCommandExecResizeResponse(bytes)
//    bytes, err = commandExecResizeResponse.Marshal()
//
//    commandExecResponse, err := UnmarshalCommandExecResponse(bytes)
//    bytes, err = commandExecResponse.Marshal()
//
//    commandExecTerminateParams, err := UnmarshalCommandExecTerminateParams(bytes)
//    bytes, err = commandExecTerminateParams.Marshal()
//
//    commandExecTerminateResponse, err := UnmarshalCommandExecTerminateResponse(bytes)
//    bytes, err = commandExecTerminateResponse.Marshal()
//
//    commandExecWriteParams, err := UnmarshalCommandExecWriteParams(bytes)
//    bytes, err = commandExecWriteParams.Marshal()
//
//    commandExecWriteResponse, err := UnmarshalCommandExecWriteResponse(bytes)
//    bytes, err = commandExecWriteResponse.Marshal()
//
//    commandExecutionOutputDeltaNotification, err := UnmarshalCommandExecutionOutputDeltaNotification(bytes)
//    bytes, err = commandExecutionOutputDeltaNotification.Marshal()
//
//    configBatchWriteParams, err := UnmarshalConfigBatchWriteParams(bytes)
//    bytes, err = configBatchWriteParams.Marshal()
//
//    configReadParams, err := UnmarshalConfigReadParams(bytes)
//    bytes, err = configReadParams.Marshal()
//
//    configReadResponse, err := UnmarshalConfigReadResponse(bytes)
//    bytes, err = configReadResponse.Marshal()
//
//    configRequirementsReadResponse, err := UnmarshalConfigRequirementsReadResponse(bytes)
//    bytes, err = configRequirementsReadResponse.Marshal()
//
//    configValueWriteParams, err := UnmarshalConfigValueWriteParams(bytes)
//    bytes, err = configValueWriteParams.Marshal()
//
//    configWarningNotification, err := UnmarshalConfigWarningNotification(bytes)
//    bytes, err = configWarningNotification.Marshal()
//
//    configWriteResponse, err := UnmarshalConfigWriteResponse(bytes)
//    bytes, err = configWriteResponse.Marshal()
//
//    contextCompactedNotification, err := UnmarshalContextCompactedNotification(bytes)
//    bytes, err = contextCompactedNotification.Marshal()
//
//    deprecationNoticeNotification, err := UnmarshalDeprecationNoticeNotification(bytes)
//    bytes, err = deprecationNoticeNotification.Marshal()
//
//    errorNotification, err := UnmarshalErrorNotification(bytes)
//    bytes, err = errorNotification.Marshal()
//
//    experimentalFeatureEnablementSetParams, err := UnmarshalExperimentalFeatureEnablementSetParams(bytes)
//    bytes, err = experimentalFeatureEnablementSetParams.Marshal()
//
//    experimentalFeatureEnablementSetResponse, err := UnmarshalExperimentalFeatureEnablementSetResponse(bytes)
//    bytes, err = experimentalFeatureEnablementSetResponse.Marshal()
//
//    experimentalFeatureListParams, err := UnmarshalExperimentalFeatureListParams(bytes)
//    bytes, err = experimentalFeatureListParams.Marshal()
//
//    experimentalFeatureListResponse, err := UnmarshalExperimentalFeatureListResponse(bytes)
//    bytes, err = experimentalFeatureListResponse.Marshal()
//
//    externalAgentConfigDetectParams, err := UnmarshalExternalAgentConfigDetectParams(bytes)
//    bytes, err = externalAgentConfigDetectParams.Marshal()
//
//    externalAgentConfigDetectResponse, err := UnmarshalExternalAgentConfigDetectResponse(bytes)
//    bytes, err = externalAgentConfigDetectResponse.Marshal()
//
//    externalAgentConfigImportCompletedNotification, err := UnmarshalExternalAgentConfigImportCompletedNotification(bytes)
//    bytes, err = externalAgentConfigImportCompletedNotification.Marshal()
//
//    externalAgentConfigImportParams, err := UnmarshalExternalAgentConfigImportParams(bytes)
//    bytes, err = externalAgentConfigImportParams.Marshal()
//
//    externalAgentConfigImportResponse, err := UnmarshalExternalAgentConfigImportResponse(bytes)
//    bytes, err = externalAgentConfigImportResponse.Marshal()
//
//    feedbackUploadParams, err := UnmarshalFeedbackUploadParams(bytes)
//    bytes, err = feedbackUploadParams.Marshal()
//
//    feedbackUploadResponse, err := UnmarshalFeedbackUploadResponse(bytes)
//    bytes, err = feedbackUploadResponse.Marshal()
//
//    fileChangeOutputDeltaNotification, err := UnmarshalFileChangeOutputDeltaNotification(bytes)
//    bytes, err = fileChangeOutputDeltaNotification.Marshal()
//
//    fileChangePatchUpdatedNotification, err := UnmarshalFileChangePatchUpdatedNotification(bytes)
//    bytes, err = fileChangePatchUpdatedNotification.Marshal()
//
//    fSChangedNotification, err := UnmarshalFSChangedNotification(bytes)
//    bytes, err = fSChangedNotification.Marshal()
//
//    fSCopyParams, err := UnmarshalFSCopyParams(bytes)
//    bytes, err = fSCopyParams.Marshal()
//
//    fSCopyResponse, err := UnmarshalFSCopyResponse(bytes)
//    bytes, err = fSCopyResponse.Marshal()
//
//    fSCreateDirectoryParams, err := UnmarshalFSCreateDirectoryParams(bytes)
//    bytes, err = fSCreateDirectoryParams.Marshal()
//
//    fSCreateDirectoryResponse, err := UnmarshalFSCreateDirectoryResponse(bytes)
//    bytes, err = fSCreateDirectoryResponse.Marshal()
//
//    fSGetMetadataParams, err := UnmarshalFSGetMetadataParams(bytes)
//    bytes, err = fSGetMetadataParams.Marshal()
//
//    fSGetMetadataResponse, err := UnmarshalFSGetMetadataResponse(bytes)
//    bytes, err = fSGetMetadataResponse.Marshal()
//
//    fSReadDirectoryParams, err := UnmarshalFSReadDirectoryParams(bytes)
//    bytes, err = fSReadDirectoryParams.Marshal()
//
//    fSReadDirectoryResponse, err := UnmarshalFSReadDirectoryResponse(bytes)
//    bytes, err = fSReadDirectoryResponse.Marshal()
//
//    fSReadFileParams, err := UnmarshalFSReadFileParams(bytes)
//    bytes, err = fSReadFileParams.Marshal()
//
//    fSReadFileResponse, err := UnmarshalFSReadFileResponse(bytes)
//    bytes, err = fSReadFileResponse.Marshal()
//
//    fSRemoveParams, err := UnmarshalFSRemoveParams(bytes)
//    bytes, err = fSRemoveParams.Marshal()
//
//    fSRemoveResponse, err := UnmarshalFSRemoveResponse(bytes)
//    bytes, err = fSRemoveResponse.Marshal()
//
//    fSUnwatchParams, err := UnmarshalFSUnwatchParams(bytes)
//    bytes, err = fSUnwatchParams.Marshal()
//
//    fSUnwatchResponse, err := UnmarshalFSUnwatchResponse(bytes)
//    bytes, err = fSUnwatchResponse.Marshal()
//
//    fSWatchParams, err := UnmarshalFSWatchParams(bytes)
//    bytes, err = fSWatchParams.Marshal()
//
//    fSWatchResponse, err := UnmarshalFSWatchResponse(bytes)
//    bytes, err = fSWatchResponse.Marshal()
//
//    fSWriteFileParams, err := UnmarshalFSWriteFileParams(bytes)
//    bytes, err = fSWriteFileParams.Marshal()
//
//    fSWriteFileResponse, err := UnmarshalFSWriteFileResponse(bytes)
//    bytes, err = fSWriteFileResponse.Marshal()
//
//    getAccountParams, err := UnmarshalGetAccountParams(bytes)
//    bytes, err = getAccountParams.Marshal()
//
//    getAccountRateLimitsResponse, err := UnmarshalGetAccountRateLimitsResponse(bytes)
//    bytes, err = getAccountRateLimitsResponse.Marshal()
//
//    getAccountResponse, err := UnmarshalGetAccountResponse(bytes)
//    bytes, err = getAccountResponse.Marshal()
//
//    guardianWarningNotification, err := UnmarshalGuardianWarningNotification(bytes)
//    bytes, err = guardianWarningNotification.Marshal()
//
//    hookCompletedNotification, err := UnmarshalHookCompletedNotification(bytes)
//    bytes, err = hookCompletedNotification.Marshal()
//
//    hookStartedNotification, err := UnmarshalHookStartedNotification(bytes)
//    bytes, err = hookStartedNotification.Marshal()
//
//    hooksListParams, err := UnmarshalHooksListParams(bytes)
//    bytes, err = hooksListParams.Marshal()
//
//    hooksListResponse, err := UnmarshalHooksListResponse(bytes)
//    bytes, err = hooksListResponse.Marshal()
//
//    itemCompletedNotification, err := UnmarshalItemCompletedNotification(bytes)
//    bytes, err = itemCompletedNotification.Marshal()
//
//    itemGuardianApprovalReviewCompletedNotification, err := UnmarshalItemGuardianApprovalReviewCompletedNotification(bytes)
//    bytes, err = itemGuardianApprovalReviewCompletedNotification.Marshal()
//
//    itemGuardianApprovalReviewStartedNotification, err := UnmarshalItemGuardianApprovalReviewStartedNotification(bytes)
//    bytes, err = itemGuardianApprovalReviewStartedNotification.Marshal()
//
//    itemStartedNotification, err := UnmarshalItemStartedNotification(bytes)
//    bytes, err = itemStartedNotification.Marshal()
//
//    listMCPServerStatusParams, err := UnmarshalListMCPServerStatusParams(bytes)
//    bytes, err = listMCPServerStatusParams.Marshal()
//
//    listMCPServerStatusResponse, err := UnmarshalListMCPServerStatusResponse(bytes)
//    bytes, err = listMCPServerStatusResponse.Marshal()
//
//    loginAccountParams, err := UnmarshalLoginAccountParams(bytes)
//    bytes, err = loginAccountParams.Marshal()
//
//    loginAccountResponse, err := UnmarshalLoginAccountResponse(bytes)
//    bytes, err = loginAccountResponse.Marshal()
//
//    logoutAccountResponse, err := UnmarshalLogoutAccountResponse(bytes)
//    bytes, err = logoutAccountResponse.Marshal()
//
//    marketplaceAddParams, err := UnmarshalMarketplaceAddParams(bytes)
//    bytes, err = marketplaceAddParams.Marshal()
//
//    marketplaceAddResponse, err := UnmarshalMarketplaceAddResponse(bytes)
//    bytes, err = marketplaceAddResponse.Marshal()
//
//    marketplaceRemoveParams, err := UnmarshalMarketplaceRemoveParams(bytes)
//    bytes, err = marketplaceRemoveParams.Marshal()
//
//    marketplaceRemoveResponse, err := UnmarshalMarketplaceRemoveResponse(bytes)
//    bytes, err = marketplaceRemoveResponse.Marshal()
//
//    marketplaceUpgradeParams, err := UnmarshalMarketplaceUpgradeParams(bytes)
//    bytes, err = marketplaceUpgradeParams.Marshal()
//
//    marketplaceUpgradeResponse, err := UnmarshalMarketplaceUpgradeResponse(bytes)
//    bytes, err = marketplaceUpgradeResponse.Marshal()
//
//    mCPResourceReadParams, err := UnmarshalMCPResourceReadParams(bytes)
//    bytes, err = mCPResourceReadParams.Marshal()
//
//    mCPResourceReadResponse, err := UnmarshalMCPResourceReadResponse(bytes)
//    bytes, err = mCPResourceReadResponse.Marshal()
//
//    mCPServerOauthLoginCompletedNotification, err := UnmarshalMCPServerOauthLoginCompletedNotification(bytes)
//    bytes, err = mCPServerOauthLoginCompletedNotification.Marshal()
//
//    mCPServerOauthLoginParams, err := UnmarshalMCPServerOauthLoginParams(bytes)
//    bytes, err = mCPServerOauthLoginParams.Marshal()
//
//    mCPServerOauthLoginResponse, err := UnmarshalMCPServerOauthLoginResponse(bytes)
//    bytes, err = mCPServerOauthLoginResponse.Marshal()
//
//    mCPServerRefreshResponse, err := UnmarshalMCPServerRefreshResponse(bytes)
//    bytes, err = mCPServerRefreshResponse.Marshal()
//
//    mCPServerStatusUpdatedNotification, err := UnmarshalMCPServerStatusUpdatedNotification(bytes)
//    bytes, err = mCPServerStatusUpdatedNotification.Marshal()
//
//    mCPServerToolCallParams, err := UnmarshalMCPServerToolCallParams(bytes)
//    bytes, err = mCPServerToolCallParams.Marshal()
//
//    mCPServerToolCallResponse, err := UnmarshalMCPServerToolCallResponse(bytes)
//    bytes, err = mCPServerToolCallResponse.Marshal()
//
//    mCPToolCallProgressNotification, err := UnmarshalMCPToolCallProgressNotification(bytes)
//    bytes, err = mCPToolCallProgressNotification.Marshal()
//
//    modelListParams, err := UnmarshalModelListParams(bytes)
//    bytes, err = modelListParams.Marshal()
//
//    modelListResponse, err := UnmarshalModelListResponse(bytes)
//    bytes, err = modelListResponse.Marshal()
//
//    modelProviderCapabilitiesReadParams, err := UnmarshalModelProviderCapabilitiesReadParams(bytes)
//    bytes, err = modelProviderCapabilitiesReadParams.Marshal()
//
//    modelProviderCapabilitiesReadResponse, err := UnmarshalModelProviderCapabilitiesReadResponse(bytes)
//    bytes, err = modelProviderCapabilitiesReadResponse.Marshal()
//
//    modelReroutedNotification, err := UnmarshalModelReroutedNotification(bytes)
//    bytes, err = modelReroutedNotification.Marshal()
//
//    modelVerificationNotification, err := UnmarshalModelVerificationNotification(bytes)
//    bytes, err = modelVerificationNotification.Marshal()
//
//    permissionProfileListParams, err := UnmarshalPermissionProfileListParams(bytes)
//    bytes, err = permissionProfileListParams.Marshal()
//
//    permissionProfileListResponse, err := UnmarshalPermissionProfileListResponse(bytes)
//    bytes, err = permissionProfileListResponse.Marshal()
//
//    planDeltaNotification, err := UnmarshalPlanDeltaNotification(bytes)
//    bytes, err = planDeltaNotification.Marshal()
//
//    pluginInstallParams, err := UnmarshalPluginInstallParams(bytes)
//    bytes, err = pluginInstallParams.Marshal()
//
//    pluginInstallResponse, err := UnmarshalPluginInstallResponse(bytes)
//    bytes, err = pluginInstallResponse.Marshal()
//
//    pluginInstalledParams, err := UnmarshalPluginInstalledParams(bytes)
//    bytes, err = pluginInstalledParams.Marshal()
//
//    pluginInstalledResponse, err := UnmarshalPluginInstalledResponse(bytes)
//    bytes, err = pluginInstalledResponse.Marshal()
//
//    pluginListParams, err := UnmarshalPluginListParams(bytes)
//    bytes, err = pluginListParams.Marshal()
//
//    pluginListResponse, err := UnmarshalPluginListResponse(bytes)
//    bytes, err = pluginListResponse.Marshal()
//
//    pluginReadParams, err := UnmarshalPluginReadParams(bytes)
//    bytes, err = pluginReadParams.Marshal()
//
//    pluginReadResponse, err := UnmarshalPluginReadResponse(bytes)
//    bytes, err = pluginReadResponse.Marshal()
//
//    pluginShareCheckoutParams, err := UnmarshalPluginShareCheckoutParams(bytes)
//    bytes, err = pluginShareCheckoutParams.Marshal()
//
//    pluginShareCheckoutResponse, err := UnmarshalPluginShareCheckoutResponse(bytes)
//    bytes, err = pluginShareCheckoutResponse.Marshal()
//
//    pluginShareDeleteParams, err := UnmarshalPluginShareDeleteParams(bytes)
//    bytes, err = pluginShareDeleteParams.Marshal()
//
//    pluginShareDeleteResponse, err := UnmarshalPluginShareDeleteResponse(bytes)
//    bytes, err = pluginShareDeleteResponse.Marshal()
//
//    pluginShareListParams, err := UnmarshalPluginShareListParams(bytes)
//    bytes, err = pluginShareListParams.Marshal()
//
//    pluginShareListResponse, err := UnmarshalPluginShareListResponse(bytes)
//    bytes, err = pluginShareListResponse.Marshal()
//
//    pluginShareSaveParams, err := UnmarshalPluginShareSaveParams(bytes)
//    bytes, err = pluginShareSaveParams.Marshal()
//
//    pluginShareSaveResponse, err := UnmarshalPluginShareSaveResponse(bytes)
//    bytes, err = pluginShareSaveResponse.Marshal()
//
//    pluginShareUpdateTargetsParams, err := UnmarshalPluginShareUpdateTargetsParams(bytes)
//    bytes, err = pluginShareUpdateTargetsParams.Marshal()
//
//    pluginShareUpdateTargetsResponse, err := UnmarshalPluginShareUpdateTargetsResponse(bytes)
//    bytes, err = pluginShareUpdateTargetsResponse.Marshal()
//
//    pluginSkillReadParams, err := UnmarshalPluginSkillReadParams(bytes)
//    bytes, err = pluginSkillReadParams.Marshal()
//
//    pluginSkillReadResponse, err := UnmarshalPluginSkillReadResponse(bytes)
//    bytes, err = pluginSkillReadResponse.Marshal()
//
//    pluginUninstallParams, err := UnmarshalPluginUninstallParams(bytes)
//    bytes, err = pluginUninstallParams.Marshal()
//
//    pluginUninstallResponse, err := UnmarshalPluginUninstallResponse(bytes)
//    bytes, err = pluginUninstallResponse.Marshal()
//
//    processExitedNotification, err := UnmarshalProcessExitedNotification(bytes)
//    bytes, err = processExitedNotification.Marshal()
//
//    processOutputDeltaNotification, err := UnmarshalProcessOutputDeltaNotification(bytes)
//    bytes, err = processOutputDeltaNotification.Marshal()
//
//    rawResponseItemCompletedNotification, err := UnmarshalRawResponseItemCompletedNotification(bytes)
//    bytes, err = rawResponseItemCompletedNotification.Marshal()
//
//    reasoningSummaryPartAddedNotification, err := UnmarshalReasoningSummaryPartAddedNotification(bytes)
//    bytes, err = reasoningSummaryPartAddedNotification.Marshal()
//
//    reasoningSummaryTextDeltaNotification, err := UnmarshalReasoningSummaryTextDeltaNotification(bytes)
//    bytes, err = reasoningSummaryTextDeltaNotification.Marshal()
//
//    reasoningTextDeltaNotification, err := UnmarshalReasoningTextDeltaNotification(bytes)
//    bytes, err = reasoningTextDeltaNotification.Marshal()
//
//    remoteControlStatusChangedNotification, err := UnmarshalRemoteControlStatusChangedNotification(bytes)
//    bytes, err = remoteControlStatusChangedNotification.Marshal()
//
//    reviewStartParams, err := UnmarshalReviewStartParams(bytes)
//    bytes, err = reviewStartParams.Marshal()
//
//    reviewStartResponse, err := UnmarshalReviewStartResponse(bytes)
//    bytes, err = reviewStartResponse.Marshal()
//
//    sendAddCreditsNudgeEmailParams, err := UnmarshalSendAddCreditsNudgeEmailParams(bytes)
//    bytes, err = sendAddCreditsNudgeEmailParams.Marshal()
//
//    sendAddCreditsNudgeEmailResponse, err := UnmarshalSendAddCreditsNudgeEmailResponse(bytes)
//    bytes, err = sendAddCreditsNudgeEmailResponse.Marshal()
//
//    serverRequestResolvedNotification, err := UnmarshalServerRequestResolvedNotification(bytes)
//    bytes, err = serverRequestResolvedNotification.Marshal()
//
//    skillsChangedNotification, err := UnmarshalSkillsChangedNotification(bytes)
//    bytes, err = skillsChangedNotification.Marshal()
//
//    skillsConfigWriteParams, err := UnmarshalSkillsConfigWriteParams(bytes)
//    bytes, err = skillsConfigWriteParams.Marshal()
//
//    skillsConfigWriteResponse, err := UnmarshalSkillsConfigWriteResponse(bytes)
//    bytes, err = skillsConfigWriteResponse.Marshal()
//
//    skillsListParams, err := UnmarshalSkillsListParams(bytes)
//    bytes, err = skillsListParams.Marshal()
//
//    skillsListResponse, err := UnmarshalSkillsListResponse(bytes)
//    bytes, err = skillsListResponse.Marshal()
//
//    terminalInteractionNotification, err := UnmarshalTerminalInteractionNotification(bytes)
//    bytes, err = terminalInteractionNotification.Marshal()
//
//    threadApproveGuardianDeniedActionParams, err := UnmarshalThreadApproveGuardianDeniedActionParams(bytes)
//    bytes, err = threadApproveGuardianDeniedActionParams.Marshal()
//
//    threadApproveGuardianDeniedActionResponse, err := UnmarshalThreadApproveGuardianDeniedActionResponse(bytes)
//    bytes, err = threadApproveGuardianDeniedActionResponse.Marshal()
//
//    threadArchiveParams, err := UnmarshalThreadArchiveParams(bytes)
//    bytes, err = threadArchiveParams.Marshal()
//
//    threadArchiveResponse, err := UnmarshalThreadArchiveResponse(bytes)
//    bytes, err = threadArchiveResponse.Marshal()
//
//    threadArchivedNotification, err := UnmarshalThreadArchivedNotification(bytes)
//    bytes, err = threadArchivedNotification.Marshal()
//
//    threadClosedNotification, err := UnmarshalThreadClosedNotification(bytes)
//    bytes, err = threadClosedNotification.Marshal()
//
//    threadCompactStartParams, err := UnmarshalThreadCompactStartParams(bytes)
//    bytes, err = threadCompactStartParams.Marshal()
//
//    threadCompactStartResponse, err := UnmarshalThreadCompactStartResponse(bytes)
//    bytes, err = threadCompactStartResponse.Marshal()
//
//    threadForkParams, err := UnmarshalThreadForkParams(bytes)
//    bytes, err = threadForkParams.Marshal()
//
//    threadForkResponse, err := UnmarshalThreadForkResponse(bytes)
//    bytes, err = threadForkResponse.Marshal()
//
//    threadGoalClearParams, err := UnmarshalThreadGoalClearParams(bytes)
//    bytes, err = threadGoalClearParams.Marshal()
//
//    threadGoalClearResponse, err := UnmarshalThreadGoalClearResponse(bytes)
//    bytes, err = threadGoalClearResponse.Marshal()
//
//    threadGoalClearedNotification, err := UnmarshalThreadGoalClearedNotification(bytes)
//    bytes, err = threadGoalClearedNotification.Marshal()
//
//    threadGoalGetParams, err := UnmarshalThreadGoalGetParams(bytes)
//    bytes, err = threadGoalGetParams.Marshal()
//
//    threadGoalGetResponse, err := UnmarshalThreadGoalGetResponse(bytes)
//    bytes, err = threadGoalGetResponse.Marshal()
//
//    threadGoalSetParams, err := UnmarshalThreadGoalSetParams(bytes)
//    bytes, err = threadGoalSetParams.Marshal()
//
//    threadGoalSetResponse, err := UnmarshalThreadGoalSetResponse(bytes)
//    bytes, err = threadGoalSetResponse.Marshal()
//
//    threadGoalUpdatedNotification, err := UnmarshalThreadGoalUpdatedNotification(bytes)
//    bytes, err = threadGoalUpdatedNotification.Marshal()
//
//    threadInjectItemsParams, err := UnmarshalThreadInjectItemsParams(bytes)
//    bytes, err = threadInjectItemsParams.Marshal()
//
//    threadInjectItemsResponse, err := UnmarshalThreadInjectItemsResponse(bytes)
//    bytes, err = threadInjectItemsResponse.Marshal()
//
//    threadListParams, err := UnmarshalThreadListParams(bytes)
//    bytes, err = threadListParams.Marshal()
//
//    threadListResponse, err := UnmarshalThreadListResponse(bytes)
//    bytes, err = threadListResponse.Marshal()
//
//    threadLoadedListParams, err := UnmarshalThreadLoadedListParams(bytes)
//    bytes, err = threadLoadedListParams.Marshal()
//
//    threadLoadedListResponse, err := UnmarshalThreadLoadedListResponse(bytes)
//    bytes, err = threadLoadedListResponse.Marshal()
//
//    threadMetadataUpdateParams, err := UnmarshalThreadMetadataUpdateParams(bytes)
//    bytes, err = threadMetadataUpdateParams.Marshal()
//
//    threadMetadataUpdateResponse, err := UnmarshalThreadMetadataUpdateResponse(bytes)
//    bytes, err = threadMetadataUpdateResponse.Marshal()
//
//    threadNameUpdatedNotification, err := UnmarshalThreadNameUpdatedNotification(bytes)
//    bytes, err = threadNameUpdatedNotification.Marshal()
//
//    threadReadParams, err := UnmarshalThreadReadParams(bytes)
//    bytes, err = threadReadParams.Marshal()
//
//    threadReadResponse, err := UnmarshalThreadReadResponse(bytes)
//    bytes, err = threadReadResponse.Marshal()
//
//    threadRealtimeClosedNotification, err := UnmarshalThreadRealtimeClosedNotification(bytes)
//    bytes, err = threadRealtimeClosedNotification.Marshal()
//
//    threadRealtimeErrorNotification, err := UnmarshalThreadRealtimeErrorNotification(bytes)
//    bytes, err = threadRealtimeErrorNotification.Marshal()
//
//    threadRealtimeItemAddedNotification, err := UnmarshalThreadRealtimeItemAddedNotification(bytes)
//    bytes, err = threadRealtimeItemAddedNotification.Marshal()
//
//    threadRealtimeOutputAudioDeltaNotification, err := UnmarshalThreadRealtimeOutputAudioDeltaNotification(bytes)
//    bytes, err = threadRealtimeOutputAudioDeltaNotification.Marshal()
//
//    threadRealtimeSDPNotification, err := UnmarshalThreadRealtimeSDPNotification(bytes)
//    bytes, err = threadRealtimeSDPNotification.Marshal()
//
//    threadRealtimeStartedNotification, err := UnmarshalThreadRealtimeStartedNotification(bytes)
//    bytes, err = threadRealtimeStartedNotification.Marshal()
//
//    threadRealtimeTranscriptDeltaNotification, err := UnmarshalThreadRealtimeTranscriptDeltaNotification(bytes)
//    bytes, err = threadRealtimeTranscriptDeltaNotification.Marshal()
//
//    threadRealtimeTranscriptDoneNotification, err := UnmarshalThreadRealtimeTranscriptDoneNotification(bytes)
//    bytes, err = threadRealtimeTranscriptDoneNotification.Marshal()
//
//    threadResumeParams, err := UnmarshalThreadResumeParams(bytes)
//    bytes, err = threadResumeParams.Marshal()
//
//    threadResumeResponse, err := UnmarshalThreadResumeResponse(bytes)
//    bytes, err = threadResumeResponse.Marshal()
//
//    threadRollbackParams, err := UnmarshalThreadRollbackParams(bytes)
//    bytes, err = threadRollbackParams.Marshal()
//
//    threadRollbackResponse, err := UnmarshalThreadRollbackResponse(bytes)
//    bytes, err = threadRollbackResponse.Marshal()
//
//    threadSetNameParams, err := UnmarshalThreadSetNameParams(bytes)
//    bytes, err = threadSetNameParams.Marshal()
//
//    threadSetNameResponse, err := UnmarshalThreadSetNameResponse(bytes)
//    bytes, err = threadSetNameResponse.Marshal()
//
//    threadSettingsUpdatedNotification, err := UnmarshalThreadSettingsUpdatedNotification(bytes)
//    bytes, err = threadSettingsUpdatedNotification.Marshal()
//
//    threadShellCommandParams, err := UnmarshalThreadShellCommandParams(bytes)
//    bytes, err = threadShellCommandParams.Marshal()
//
//    threadShellCommandResponse, err := UnmarshalThreadShellCommandResponse(bytes)
//    bytes, err = threadShellCommandResponse.Marshal()
//
//    threadStartParams, err := UnmarshalThreadStartParams(bytes)
//    bytes, err = threadStartParams.Marshal()
//
//    threadStartResponse, err := UnmarshalThreadStartResponse(bytes)
//    bytes, err = threadStartResponse.Marshal()
//
//    threadStartedNotification, err := UnmarshalThreadStartedNotification(bytes)
//    bytes, err = threadStartedNotification.Marshal()
//
//    threadStatusChangedNotification, err := UnmarshalThreadStatusChangedNotification(bytes)
//    bytes, err = threadStatusChangedNotification.Marshal()
//
//    threadTokenUsageUpdatedNotification, err := UnmarshalThreadTokenUsageUpdatedNotification(bytes)
//    bytes, err = threadTokenUsageUpdatedNotification.Marshal()
//
//    threadUnarchiveParams, err := UnmarshalThreadUnarchiveParams(bytes)
//    bytes, err = threadUnarchiveParams.Marshal()
//
//    threadUnarchiveResponse, err := UnmarshalThreadUnarchiveResponse(bytes)
//    bytes, err = threadUnarchiveResponse.Marshal()
//
//    threadUnarchivedNotification, err := UnmarshalThreadUnarchivedNotification(bytes)
//    bytes, err = threadUnarchivedNotification.Marshal()
//
//    threadUnsubscribeParams, err := UnmarshalThreadUnsubscribeParams(bytes)
//    bytes, err = threadUnsubscribeParams.Marshal()
//
//    threadUnsubscribeResponse, err := UnmarshalThreadUnsubscribeResponse(bytes)
//    bytes, err = threadUnsubscribeResponse.Marshal()
//
//    turnCompletedNotification, err := UnmarshalTurnCompletedNotification(bytes)
//    bytes, err = turnCompletedNotification.Marshal()
//
//    turnDiffUpdatedNotification, err := UnmarshalTurnDiffUpdatedNotification(bytes)
//    bytes, err = turnDiffUpdatedNotification.Marshal()
//
//    turnInterruptParams, err := UnmarshalTurnInterruptParams(bytes)
//    bytes, err = turnInterruptParams.Marshal()
//
//    turnInterruptResponse, err := UnmarshalTurnInterruptResponse(bytes)
//    bytes, err = turnInterruptResponse.Marshal()
//
//    turnPlanUpdatedNotification, err := UnmarshalTurnPlanUpdatedNotification(bytes)
//    bytes, err = turnPlanUpdatedNotification.Marshal()
//
//    turnStartParams, err := UnmarshalTurnStartParams(bytes)
//    bytes, err = turnStartParams.Marshal()
//
//    turnStartResponse, err := UnmarshalTurnStartResponse(bytes)
//    bytes, err = turnStartResponse.Marshal()
//
//    turnStartedNotification, err := UnmarshalTurnStartedNotification(bytes)
//    bytes, err = turnStartedNotification.Marshal()
//
//    turnSteerParams, err := UnmarshalTurnSteerParams(bytes)
//    bytes, err = turnSteerParams.Marshal()
//
//    turnSteerResponse, err := UnmarshalTurnSteerResponse(bytes)
//    bytes, err = turnSteerResponse.Marshal()
//
//    warningNotification, err := UnmarshalWarningNotification(bytes)
//    bytes, err = warningNotification.Marshal()
//
//    windowsSandboxReadinessResponse, err := UnmarshalWindowsSandboxReadinessResponse(bytes)
//    bytes, err = windowsSandboxReadinessResponse.Marshal()
//
//    windowsSandboxSetupCompletedNotification, err := UnmarshalWindowsSandboxSetupCompletedNotification(bytes)
//    bytes, err = windowsSandboxSetupCompletedNotification.Marshal()
//
//    windowsSandboxSetupStartParams, err := UnmarshalWindowsSandboxSetupStartParams(bytes)
//    bytes, err = windowsSandboxSetupStartParams.Marshal()
//
//    windowsSandboxSetupStartResponse, err := UnmarshalWindowsSandboxSetupStartResponse(bytes)
//    bytes, err = windowsSandboxSetupStartResponse.Marshal()
//
//    windowsWorldWritableWarningNotification, err := UnmarshalWindowsWorldWritableWarningNotification(bytes)
//    bytes, err = windowsWorldWritableWarningNotification.Marshal()

package codexschemav2

import "bytes"
import "errors"

import "encoding/json"

func UnmarshalAccountLoginCompletedNotification(data []byte) (AccountLoginCompletedNotification, error) {
	var r AccountLoginCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AccountLoginCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAccountRateLimitsUpdatedNotification(data []byte) (AccountRateLimitsUpdatedNotification, error) {
	var r AccountRateLimitsUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AccountRateLimitsUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAccountUpdatedNotification(data []byte) (AccountUpdatedNotification, error) {
	var r AccountUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AccountUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAgentMessageDeltaNotification(data []byte) (AgentMessageDeltaNotification, error) {
	var r AgentMessageDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AgentMessageDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAppListUpdatedNotification(data []byte) (AppListUpdatedNotification, error) {
	var r AppListUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AppListUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAppsListParams(data []byte) (AppsListParams, error) {
	var r AppsListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AppsListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAppsListResponse(data []byte) (AppsListResponse, error) {
	var r AppsListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AppsListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCancelLoginAccountParams(data []byte) (CancelLoginAccountParams, error) {
	var r CancelLoginAccountParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CancelLoginAccountParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCancelLoginAccountResponse(data []byte) (CancelLoginAccountResponse, error) {
	var r CancelLoginAccountResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CancelLoginAccountResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecOutputDeltaNotification(data []byte) (CommandExecOutputDeltaNotification, error) {
	var r CommandExecOutputDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecOutputDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecParams(data []byte) (CommandExecParams, error) {
	var r CommandExecParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecResizeParams(data []byte) (CommandExecResizeParams, error) {
	var r CommandExecResizeParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecResizeParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type CommandExecResizeResponse map[string]interface{}

func UnmarshalCommandExecResizeResponse(data []byte) (CommandExecResizeResponse, error) {
	var r CommandExecResizeResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecResizeResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecResponse(data []byte) (CommandExecResponse, error) {
	var r CommandExecResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecTerminateParams(data []byte) (CommandExecTerminateParams, error) {
	var r CommandExecTerminateParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecTerminateParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type CommandExecTerminateResponse map[string]interface{}

func UnmarshalCommandExecTerminateResponse(data []byte) (CommandExecTerminateResponse, error) {
	var r CommandExecTerminateResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecTerminateResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecWriteParams(data []byte) (CommandExecWriteParams, error) {
	var r CommandExecWriteParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecWriteParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type CommandExecWriteResponse map[string]interface{}

func UnmarshalCommandExecWriteResponse(data []byte) (CommandExecWriteResponse, error) {
	var r CommandExecWriteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecWriteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommandExecutionOutputDeltaNotification(data []byte) (CommandExecutionOutputDeltaNotification, error) {
	var r CommandExecutionOutputDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CommandExecutionOutputDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigBatchWriteParams(data []byte) (ConfigBatchWriteParams, error) {
	var r ConfigBatchWriteParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigBatchWriteParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigReadParams(data []byte) (ConfigReadParams, error) {
	var r ConfigReadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigReadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigReadResponse(data []byte) (ConfigReadResponse, error) {
	var r ConfigReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigRequirementsReadResponse(data []byte) (ConfigRequirementsReadResponse, error) {
	var r ConfigRequirementsReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigRequirementsReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigValueWriteParams(data []byte) (ConfigValueWriteParams, error) {
	var r ConfigValueWriteParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigValueWriteParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigWarningNotification(data []byte) (ConfigWarningNotification, error) {
	var r ConfigWarningNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigWarningNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalConfigWriteResponse(data []byte) (ConfigWriteResponse, error) {
	var r ConfigWriteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ConfigWriteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalContextCompactedNotification(data []byte) (ContextCompactedNotification, error) {
	var r ContextCompactedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ContextCompactedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalDeprecationNoticeNotification(data []byte) (DeprecationNoticeNotification, error) {
	var r DeprecationNoticeNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *DeprecationNoticeNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalErrorNotification(data []byte) (ErrorNotification, error) {
	var r ErrorNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ErrorNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExperimentalFeatureEnablementSetParams(data []byte) (ExperimentalFeatureEnablementSetParams, error) {
	var r ExperimentalFeatureEnablementSetParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExperimentalFeatureEnablementSetParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExperimentalFeatureEnablementSetResponse(data []byte) (ExperimentalFeatureEnablementSetResponse, error) {
	var r ExperimentalFeatureEnablementSetResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExperimentalFeatureEnablementSetResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExperimentalFeatureListParams(data []byte) (ExperimentalFeatureListParams, error) {
	var r ExperimentalFeatureListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExperimentalFeatureListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExperimentalFeatureListResponse(data []byte) (ExperimentalFeatureListResponse, error) {
	var r ExperimentalFeatureListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExperimentalFeatureListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExternalAgentConfigDetectParams(data []byte) (ExternalAgentConfigDetectParams, error) {
	var r ExternalAgentConfigDetectParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExternalAgentConfigDetectParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExternalAgentConfigDetectResponse(data []byte) (ExternalAgentConfigDetectResponse, error) {
	var r ExternalAgentConfigDetectResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExternalAgentConfigDetectResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ExternalAgentConfigImportCompletedNotification map[string]interface{}

func UnmarshalExternalAgentConfigImportCompletedNotification(data []byte) (ExternalAgentConfigImportCompletedNotification, error) {
	var r ExternalAgentConfigImportCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExternalAgentConfigImportCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalExternalAgentConfigImportParams(data []byte) (ExternalAgentConfigImportParams, error) {
	var r ExternalAgentConfigImportParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExternalAgentConfigImportParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ExternalAgentConfigImportResponse map[string]interface{}

func UnmarshalExternalAgentConfigImportResponse(data []byte) (ExternalAgentConfigImportResponse, error) {
	var r ExternalAgentConfigImportResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ExternalAgentConfigImportResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFeedbackUploadParams(data []byte) (FeedbackUploadParams, error) {
	var r FeedbackUploadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FeedbackUploadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFeedbackUploadResponse(data []byte) (FeedbackUploadResponse, error) {
	var r FeedbackUploadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FeedbackUploadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFileChangeOutputDeltaNotification(data []byte) (FileChangeOutputDeltaNotification, error) {
	var r FileChangeOutputDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FileChangeOutputDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFileChangePatchUpdatedNotification(data []byte) (FileChangePatchUpdatedNotification, error) {
	var r FileChangePatchUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FileChangePatchUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSChangedNotification(data []byte) (FSChangedNotification, error) {
	var r FSChangedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSChangedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSCopyParams(data []byte) (FSCopyParams, error) {
	var r FSCopyParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSCopyParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type FSCopyResponse map[string]interface{}

func UnmarshalFSCopyResponse(data []byte) (FSCopyResponse, error) {
	var r FSCopyResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSCopyResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSCreateDirectoryParams(data []byte) (FSCreateDirectoryParams, error) {
	var r FSCreateDirectoryParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSCreateDirectoryParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type FSCreateDirectoryResponse map[string]interface{}

func UnmarshalFSCreateDirectoryResponse(data []byte) (FSCreateDirectoryResponse, error) {
	var r FSCreateDirectoryResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSCreateDirectoryResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSGetMetadataParams(data []byte) (FSGetMetadataParams, error) {
	var r FSGetMetadataParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSGetMetadataParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSGetMetadataResponse(data []byte) (FSGetMetadataResponse, error) {
	var r FSGetMetadataResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSGetMetadataResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSReadDirectoryParams(data []byte) (FSReadDirectoryParams, error) {
	var r FSReadDirectoryParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSReadDirectoryParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSReadDirectoryResponse(data []byte) (FSReadDirectoryResponse, error) {
	var r FSReadDirectoryResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSReadDirectoryResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSReadFileParams(data []byte) (FSReadFileParams, error) {
	var r FSReadFileParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSReadFileParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSReadFileResponse(data []byte) (FSReadFileResponse, error) {
	var r FSReadFileResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSReadFileResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSRemoveParams(data []byte) (FSRemoveParams, error) {
	var r FSRemoveParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSRemoveParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type FSRemoveResponse map[string]interface{}

func UnmarshalFSRemoveResponse(data []byte) (FSRemoveResponse, error) {
	var r FSRemoveResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSRemoveResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSUnwatchParams(data []byte) (FSUnwatchParams, error) {
	var r FSUnwatchParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSUnwatchParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type FSUnwatchResponse map[string]interface{}

func UnmarshalFSUnwatchResponse(data []byte) (FSUnwatchResponse, error) {
	var r FSUnwatchResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSUnwatchResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSWatchParams(data []byte) (FSWatchParams, error) {
	var r FSWatchParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSWatchParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSWatchResponse(data []byte) (FSWatchResponse, error) {
	var r FSWatchResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSWatchResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFSWriteFileParams(data []byte) (FSWriteFileParams, error) {
	var r FSWriteFileParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSWriteFileParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type FSWriteFileResponse map[string]interface{}

func UnmarshalFSWriteFileResponse(data []byte) (FSWriteFileResponse, error) {
	var r FSWriteFileResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FSWriteFileResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGetAccountParams(data []byte) (GetAccountParams, error) {
	var r GetAccountParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GetAccountParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGetAccountRateLimitsResponse(data []byte) (GetAccountRateLimitsResponse, error) {
	var r GetAccountRateLimitsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GetAccountRateLimitsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGetAccountResponse(data []byte) (GetAccountResponse, error) {
	var r GetAccountResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GetAccountResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGuardianWarningNotification(data []byte) (GuardianWarningNotification, error) {
	var r GuardianWarningNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GuardianWarningNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalHookCompletedNotification(data []byte) (HookCompletedNotification, error) {
	var r HookCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *HookCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalHookStartedNotification(data []byte) (HookStartedNotification, error) {
	var r HookStartedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *HookStartedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalHooksListParams(data []byte) (HooksListParams, error) {
	var r HooksListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *HooksListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalHooksListResponse(data []byte) (HooksListResponse, error) {
	var r HooksListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *HooksListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalItemCompletedNotification(data []byte) (ItemCompletedNotification, error) {
	var r ItemCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ItemCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalItemGuardianApprovalReviewCompletedNotification(data []byte) (ItemGuardianApprovalReviewCompletedNotification, error) {
	var r ItemGuardianApprovalReviewCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ItemGuardianApprovalReviewCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalItemGuardianApprovalReviewStartedNotification(data []byte) (ItemGuardianApprovalReviewStartedNotification, error) {
	var r ItemGuardianApprovalReviewStartedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ItemGuardianApprovalReviewStartedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalItemStartedNotification(data []byte) (ItemStartedNotification, error) {
	var r ItemStartedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ItemStartedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalListMCPServerStatusParams(data []byte) (ListMCPServerStatusParams, error) {
	var r ListMCPServerStatusParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ListMCPServerStatusParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalListMCPServerStatusResponse(data []byte) (ListMCPServerStatusResponse, error) {
	var r ListMCPServerStatusResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ListMCPServerStatusResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLoginAccountParams(data []byte) (LoginAccountParams, error) {
	var r LoginAccountParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LoginAccountParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLoginAccountResponse(data []byte) (LoginAccountResponse, error) {
	var r LoginAccountResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LoginAccountResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type LogoutAccountResponse map[string]interface{}

func UnmarshalLogoutAccountResponse(data []byte) (LogoutAccountResponse, error) {
	var r LogoutAccountResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LogoutAccountResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMarketplaceAddParams(data []byte) (MarketplaceAddParams, error) {
	var r MarketplaceAddParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MarketplaceAddParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMarketplaceAddResponse(data []byte) (MarketplaceAddResponse, error) {
	var r MarketplaceAddResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MarketplaceAddResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMarketplaceRemoveParams(data []byte) (MarketplaceRemoveParams, error) {
	var r MarketplaceRemoveParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MarketplaceRemoveParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMarketplaceRemoveResponse(data []byte) (MarketplaceRemoveResponse, error) {
	var r MarketplaceRemoveResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MarketplaceRemoveResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMarketplaceUpgradeParams(data []byte) (MarketplaceUpgradeParams, error) {
	var r MarketplaceUpgradeParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MarketplaceUpgradeParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMarketplaceUpgradeResponse(data []byte) (MarketplaceUpgradeResponse, error) {
	var r MarketplaceUpgradeResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MarketplaceUpgradeResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPResourceReadParams(data []byte) (MCPResourceReadParams, error) {
	var r MCPResourceReadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPResourceReadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPResourceReadResponse(data []byte) (MCPResourceReadResponse, error) {
	var r MCPResourceReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPResourceReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPServerOauthLoginCompletedNotification(data []byte) (MCPServerOauthLoginCompletedNotification, error) {
	var r MCPServerOauthLoginCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerOauthLoginCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPServerOauthLoginParams(data []byte) (MCPServerOauthLoginParams, error) {
	var r MCPServerOauthLoginParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerOauthLoginParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPServerOauthLoginResponse(data []byte) (MCPServerOauthLoginResponse, error) {
	var r MCPServerOauthLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerOauthLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type MCPServerRefreshResponse map[string]interface{}

func UnmarshalMCPServerRefreshResponse(data []byte) (MCPServerRefreshResponse, error) {
	var r MCPServerRefreshResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerRefreshResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPServerStatusUpdatedNotification(data []byte) (MCPServerStatusUpdatedNotification, error) {
	var r MCPServerStatusUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerStatusUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPServerToolCallParams(data []byte) (MCPServerToolCallParams, error) {
	var r MCPServerToolCallParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerToolCallParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPServerToolCallResponse(data []byte) (MCPServerToolCallResponse, error) {
	var r MCPServerToolCallResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPServerToolCallResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMCPToolCallProgressNotification(data []byte) (MCPToolCallProgressNotification, error) {
	var r MCPToolCallProgressNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MCPToolCallProgressNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalModelListParams(data []byte) (ModelListParams, error) {
	var r ModelListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ModelListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalModelListResponse(data []byte) (ModelListResponse, error) {
	var r ModelListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ModelListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ModelProviderCapabilitiesReadParams map[string]interface{}

func UnmarshalModelProviderCapabilitiesReadParams(data []byte) (ModelProviderCapabilitiesReadParams, error) {
	var r ModelProviderCapabilitiesReadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ModelProviderCapabilitiesReadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalModelProviderCapabilitiesReadResponse(data []byte) (ModelProviderCapabilitiesReadResponse, error) {
	var r ModelProviderCapabilitiesReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ModelProviderCapabilitiesReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalModelReroutedNotification(data []byte) (ModelReroutedNotification, error) {
	var r ModelReroutedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ModelReroutedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalModelVerificationNotification(data []byte) (ModelVerificationNotification, error) {
	var r ModelVerificationNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ModelVerificationNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPermissionProfileListParams(data []byte) (PermissionProfileListParams, error) {
	var r PermissionProfileListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PermissionProfileListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPermissionProfileListResponse(data []byte) (PermissionProfileListResponse, error) {
	var r PermissionProfileListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PermissionProfileListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPlanDeltaNotification(data []byte) (PlanDeltaNotification, error) {
	var r PlanDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PlanDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginInstallParams(data []byte) (PluginInstallParams, error) {
	var r PluginInstallParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginInstallParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginInstallResponse(data []byte) (PluginInstallResponse, error) {
	var r PluginInstallResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginInstallResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginInstalledParams(data []byte) (PluginInstalledParams, error) {
	var r PluginInstalledParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginInstalledParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginInstalledResponse(data []byte) (PluginInstalledResponse, error) {
	var r PluginInstalledResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginInstalledResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginListParams(data []byte) (PluginListParams, error) {
	var r PluginListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginListResponse(data []byte) (PluginListResponse, error) {
	var r PluginListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginReadParams(data []byte) (PluginReadParams, error) {
	var r PluginReadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginReadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginReadResponse(data []byte) (PluginReadResponse, error) {
	var r PluginReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareCheckoutParams(data []byte) (PluginShareCheckoutParams, error) {
	var r PluginShareCheckoutParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareCheckoutParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareCheckoutResponse(data []byte) (PluginShareCheckoutResponse, error) {
	var r PluginShareCheckoutResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareCheckoutResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareDeleteParams(data []byte) (PluginShareDeleteParams, error) {
	var r PluginShareDeleteParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareDeleteParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type PluginShareDeleteResponse map[string]interface{}

func UnmarshalPluginShareDeleteResponse(data []byte) (PluginShareDeleteResponse, error) {
	var r PluginShareDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type PluginShareListParams map[string]interface{}

func UnmarshalPluginShareListParams(data []byte) (PluginShareListParams, error) {
	var r PluginShareListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareListResponse(data []byte) (PluginShareListResponse, error) {
	var r PluginShareListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareSaveParams(data []byte) (PluginShareSaveParams, error) {
	var r PluginShareSaveParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareSaveParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareSaveResponse(data []byte) (PluginShareSaveResponse, error) {
	var r PluginShareSaveResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareSaveResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareUpdateTargetsParams(data []byte) (PluginShareUpdateTargetsParams, error) {
	var r PluginShareUpdateTargetsParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareUpdateTargetsParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginShareUpdateTargetsResponse(data []byte) (PluginShareUpdateTargetsResponse, error) {
	var r PluginShareUpdateTargetsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginShareUpdateTargetsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginSkillReadParams(data []byte) (PluginSkillReadParams, error) {
	var r PluginSkillReadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginSkillReadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginSkillReadResponse(data []byte) (PluginSkillReadResponse, error) {
	var r PluginSkillReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginSkillReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPluginUninstallParams(data []byte) (PluginUninstallParams, error) {
	var r PluginUninstallParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginUninstallParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type PluginUninstallResponse map[string]interface{}

func UnmarshalPluginUninstallResponse(data []byte) (PluginUninstallResponse, error) {
	var r PluginUninstallResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PluginUninstallResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProcessExitedNotification(data []byte) (ProcessExitedNotification, error) {
	var r ProcessExitedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProcessExitedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProcessOutputDeltaNotification(data []byte) (ProcessOutputDeltaNotification, error) {
	var r ProcessOutputDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProcessOutputDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalRawResponseItemCompletedNotification(data []byte) (RawResponseItemCompletedNotification, error) {
	var r RawResponseItemCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *RawResponseItemCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalReasoningSummaryPartAddedNotification(data []byte) (ReasoningSummaryPartAddedNotification, error) {
	var r ReasoningSummaryPartAddedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ReasoningSummaryPartAddedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalReasoningSummaryTextDeltaNotification(data []byte) (ReasoningSummaryTextDeltaNotification, error) {
	var r ReasoningSummaryTextDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ReasoningSummaryTextDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalReasoningTextDeltaNotification(data []byte) (ReasoningTextDeltaNotification, error) {
	var r ReasoningTextDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ReasoningTextDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalRemoteControlStatusChangedNotification(data []byte) (RemoteControlStatusChangedNotification, error) {
	var r RemoteControlStatusChangedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *RemoteControlStatusChangedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalReviewStartParams(data []byte) (ReviewStartParams, error) {
	var r ReviewStartParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ReviewStartParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalReviewStartResponse(data []byte) (ReviewStartResponse, error) {
	var r ReviewStartResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ReviewStartResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSendAddCreditsNudgeEmailParams(data []byte) (SendAddCreditsNudgeEmailParams, error) {
	var r SendAddCreditsNudgeEmailParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SendAddCreditsNudgeEmailParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSendAddCreditsNudgeEmailResponse(data []byte) (SendAddCreditsNudgeEmailResponse, error) {
	var r SendAddCreditsNudgeEmailResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SendAddCreditsNudgeEmailResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalServerRequestResolvedNotification(data []byte) (ServerRequestResolvedNotification, error) {
	var r ServerRequestResolvedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ServerRequestResolvedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type SkillsChangedNotification map[string]interface{}

func UnmarshalSkillsChangedNotification(data []byte) (SkillsChangedNotification, error) {
	var r SkillsChangedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SkillsChangedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSkillsConfigWriteParams(data []byte) (SkillsConfigWriteParams, error) {
	var r SkillsConfigWriteParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SkillsConfigWriteParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSkillsConfigWriteResponse(data []byte) (SkillsConfigWriteResponse, error) {
	var r SkillsConfigWriteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SkillsConfigWriteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSkillsListParams(data []byte) (SkillsListParams, error) {
	var r SkillsListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SkillsListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSkillsListResponse(data []byte) (SkillsListResponse, error) {
	var r SkillsListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SkillsListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTerminalInteractionNotification(data []byte) (TerminalInteractionNotification, error) {
	var r TerminalInteractionNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TerminalInteractionNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadApproveGuardianDeniedActionParams(data []byte) (ThreadApproveGuardianDeniedActionParams, error) {
	var r ThreadApproveGuardianDeniedActionParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadApproveGuardianDeniedActionParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ThreadApproveGuardianDeniedActionResponse map[string]interface{}

func UnmarshalThreadApproveGuardianDeniedActionResponse(data []byte) (ThreadApproveGuardianDeniedActionResponse, error) {
	var r ThreadApproveGuardianDeniedActionResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadApproveGuardianDeniedActionResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadArchiveParams(data []byte) (ThreadArchiveParams, error) {
	var r ThreadArchiveParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadArchiveParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ThreadArchiveResponse map[string]interface{}

func UnmarshalThreadArchiveResponse(data []byte) (ThreadArchiveResponse, error) {
	var r ThreadArchiveResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadArchiveResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadArchivedNotification(data []byte) (ThreadArchivedNotification, error) {
	var r ThreadArchivedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadArchivedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadClosedNotification(data []byte) (ThreadClosedNotification, error) {
	var r ThreadClosedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadClosedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadCompactStartParams(data []byte) (ThreadCompactStartParams, error) {
	var r ThreadCompactStartParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadCompactStartParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ThreadCompactStartResponse map[string]interface{}

func UnmarshalThreadCompactStartResponse(data []byte) (ThreadCompactStartResponse, error) {
	var r ThreadCompactStartResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadCompactStartResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadForkParams(data []byte) (ThreadForkParams, error) {
	var r ThreadForkParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadForkParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadForkResponse(data []byte) (ThreadForkResponse, error) {
	var r ThreadForkResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadForkResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalClearParams(data []byte) (ThreadGoalClearParams, error) {
	var r ThreadGoalClearParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalClearParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalClearResponse(data []byte) (ThreadGoalClearResponse, error) {
	var r ThreadGoalClearResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalClearResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalClearedNotification(data []byte) (ThreadGoalClearedNotification, error) {
	var r ThreadGoalClearedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalClearedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalGetParams(data []byte) (ThreadGoalGetParams, error) {
	var r ThreadGoalGetParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalGetParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalGetResponse(data []byte) (ThreadGoalGetResponse, error) {
	var r ThreadGoalGetResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalGetResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalSetParams(data []byte) (ThreadGoalSetParams, error) {
	var r ThreadGoalSetParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalSetParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalSetResponse(data []byte) (ThreadGoalSetResponse, error) {
	var r ThreadGoalSetResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalSetResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadGoalUpdatedNotification(data []byte) (ThreadGoalUpdatedNotification, error) {
	var r ThreadGoalUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadGoalUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadInjectItemsParams(data []byte) (ThreadInjectItemsParams, error) {
	var r ThreadInjectItemsParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadInjectItemsParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ThreadInjectItemsResponse map[string]interface{}

func UnmarshalThreadInjectItemsResponse(data []byte) (ThreadInjectItemsResponse, error) {
	var r ThreadInjectItemsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadInjectItemsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadListParams(data []byte) (ThreadListParams, error) {
	var r ThreadListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadListResponse(data []byte) (ThreadListResponse, error) {
	var r ThreadListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadLoadedListParams(data []byte) (ThreadLoadedListParams, error) {
	var r ThreadLoadedListParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadLoadedListParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadLoadedListResponse(data []byte) (ThreadLoadedListResponse, error) {
	var r ThreadLoadedListResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadLoadedListResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadMetadataUpdateParams(data []byte) (ThreadMetadataUpdateParams, error) {
	var r ThreadMetadataUpdateParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadMetadataUpdateParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadMetadataUpdateResponse(data []byte) (ThreadMetadataUpdateResponse, error) {
	var r ThreadMetadataUpdateResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadMetadataUpdateResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadNameUpdatedNotification(data []byte) (ThreadNameUpdatedNotification, error) {
	var r ThreadNameUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadNameUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadReadParams(data []byte) (ThreadReadParams, error) {
	var r ThreadReadParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadReadParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadReadResponse(data []byte) (ThreadReadResponse, error) {
	var r ThreadReadResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadReadResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeClosedNotification(data []byte) (ThreadRealtimeClosedNotification, error) {
	var r ThreadRealtimeClosedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeClosedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeErrorNotification(data []byte) (ThreadRealtimeErrorNotification, error) {
	var r ThreadRealtimeErrorNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeErrorNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeItemAddedNotification(data []byte) (ThreadRealtimeItemAddedNotification, error) {
	var r ThreadRealtimeItemAddedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeItemAddedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeOutputAudioDeltaNotification(data []byte) (ThreadRealtimeOutputAudioDeltaNotification, error) {
	var r ThreadRealtimeOutputAudioDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeOutputAudioDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeSDPNotification(data []byte) (ThreadRealtimeSDPNotification, error) {
	var r ThreadRealtimeSDPNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeSDPNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeStartedNotification(data []byte) (ThreadRealtimeStartedNotification, error) {
	var r ThreadRealtimeStartedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeStartedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeTranscriptDeltaNotification(data []byte) (ThreadRealtimeTranscriptDeltaNotification, error) {
	var r ThreadRealtimeTranscriptDeltaNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeTranscriptDeltaNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRealtimeTranscriptDoneNotification(data []byte) (ThreadRealtimeTranscriptDoneNotification, error) {
	var r ThreadRealtimeTranscriptDoneNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRealtimeTranscriptDoneNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadResumeParams(data []byte) (ThreadResumeParams, error) {
	var r ThreadResumeParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadResumeParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadResumeResponse(data []byte) (ThreadResumeResponse, error) {
	var r ThreadResumeResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadResumeResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRollbackParams(data []byte) (ThreadRollbackParams, error) {
	var r ThreadRollbackParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRollbackParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadRollbackResponse(data []byte) (ThreadRollbackResponse, error) {
	var r ThreadRollbackResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadRollbackResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadSetNameParams(data []byte) (ThreadSetNameParams, error) {
	var r ThreadSetNameParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadSetNameParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ThreadSetNameResponse map[string]interface{}

func UnmarshalThreadSetNameResponse(data []byte) (ThreadSetNameResponse, error) {
	var r ThreadSetNameResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadSetNameResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadSettingsUpdatedNotification(data []byte) (ThreadSettingsUpdatedNotification, error) {
	var r ThreadSettingsUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadSettingsUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadShellCommandParams(data []byte) (ThreadShellCommandParams, error) {
	var r ThreadShellCommandParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadShellCommandParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ThreadShellCommandResponse map[string]interface{}

func UnmarshalThreadShellCommandResponse(data []byte) (ThreadShellCommandResponse, error) {
	var r ThreadShellCommandResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadShellCommandResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadStartParams(data []byte) (ThreadStartParams, error) {
	var r ThreadStartParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadStartParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadStartResponse(data []byte) (ThreadStartResponse, error) {
	var r ThreadStartResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadStartResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadStartedNotification(data []byte) (ThreadStartedNotification, error) {
	var r ThreadStartedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadStartedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadStatusChangedNotification(data []byte) (ThreadStatusChangedNotification, error) {
	var r ThreadStatusChangedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadStatusChangedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadTokenUsageUpdatedNotification(data []byte) (ThreadTokenUsageUpdatedNotification, error) {
	var r ThreadTokenUsageUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadTokenUsageUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadUnarchiveParams(data []byte) (ThreadUnarchiveParams, error) {
	var r ThreadUnarchiveParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadUnarchiveParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadUnarchiveResponse(data []byte) (ThreadUnarchiveResponse, error) {
	var r ThreadUnarchiveResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadUnarchiveResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadUnarchivedNotification(data []byte) (ThreadUnarchivedNotification, error) {
	var r ThreadUnarchivedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadUnarchivedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadUnsubscribeParams(data []byte) (ThreadUnsubscribeParams, error) {
	var r ThreadUnsubscribeParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadUnsubscribeParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalThreadUnsubscribeResponse(data []byte) (ThreadUnsubscribeResponse, error) {
	var r ThreadUnsubscribeResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ThreadUnsubscribeResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnCompletedNotification(data []byte) (TurnCompletedNotification, error) {
	var r TurnCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnDiffUpdatedNotification(data []byte) (TurnDiffUpdatedNotification, error) {
	var r TurnDiffUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnDiffUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnInterruptParams(data []byte) (TurnInterruptParams, error) {
	var r TurnInterruptParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnInterruptParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type TurnInterruptResponse map[string]interface{}

func UnmarshalTurnInterruptResponse(data []byte) (TurnInterruptResponse, error) {
	var r TurnInterruptResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnInterruptResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnPlanUpdatedNotification(data []byte) (TurnPlanUpdatedNotification, error) {
	var r TurnPlanUpdatedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnPlanUpdatedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnStartParams(data []byte) (TurnStartParams, error) {
	var r TurnStartParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnStartParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnStartResponse(data []byte) (TurnStartResponse, error) {
	var r TurnStartResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnStartResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnStartedNotification(data []byte) (TurnStartedNotification, error) {
	var r TurnStartedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnStartedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnSteerParams(data []byte) (TurnSteerParams, error) {
	var r TurnSteerParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnSteerParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTurnSteerResponse(data []byte) (TurnSteerResponse, error) {
	var r TurnSteerResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TurnSteerResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWarningNotification(data []byte) (WarningNotification, error) {
	var r WarningNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WarningNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWindowsSandboxReadinessResponse(data []byte) (WindowsSandboxReadinessResponse, error) {
	var r WindowsSandboxReadinessResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WindowsSandboxReadinessResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWindowsSandboxSetupCompletedNotification(data []byte) (WindowsSandboxSetupCompletedNotification, error) {
	var r WindowsSandboxSetupCompletedNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WindowsSandboxSetupCompletedNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWindowsSandboxSetupStartParams(data []byte) (WindowsSandboxSetupStartParams, error) {
	var r WindowsSandboxSetupStartParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WindowsSandboxSetupStartParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWindowsSandboxSetupStartResponse(data []byte) (WindowsSandboxSetupStartResponse, error) {
	var r WindowsSandboxSetupStartResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WindowsSandboxSetupStartResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWindowsWorldWritableWarningNotification(data []byte) (WindowsWorldWritableWarningNotification, error) {
	var r WindowsWorldWritableWarningNotification
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WindowsWorldWritableWarningNotification) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type AccountLoginCompletedNotification struct {
	Error   *string `json:"error"`
	LoginID *string `json:"loginId"`
	Success bool    `json:"success"`
}

type AccountRateLimitsUpdatedNotification struct {
	RateLimits AccountRateLimitsUpdatedNotificationRateLimits `json:"rateLimits"`
}

type AccountRateLimitsUpdatedNotificationRateLimits struct {
	Credits              *PurpleCreditsSnapshot `json:"credits"`
	LimitID              *string                `json:"limitId"`
	LimitName            *string                `json:"limitName"`
	PlanType             *PlanType              `json:"planType"`
	Primary              *PurpleRateLimitWindow `json:"primary"`
	RateLimitReachedType *RateLimitReachedType  `json:"rateLimitReachedType"`
	Secondary            *PurpleRateLimitWindow `json:"secondary"`
}

type PurpleCreditsSnapshot struct {
	Balance    *string `json:"balance"`
	HasCredits bool    `json:"hasCredits"`
	Unlimited  bool    `json:"unlimited"`
}

type PurpleRateLimitWindow struct {
	ResetsAt           *int64 `json:"resetsAt"`
	UsedPercent        int64  `json:"usedPercent"`
	WindowDurationMins *int64 `json:"windowDurationMins"`
}

type AccountUpdatedNotification struct {
	AuthMode *AuthMode `json:"authMode"`
	PlanType *PlanType `json:"planType"`
}

type AgentMessageDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// EXPERIMENTAL - notification emitted when the app list changes.
type AppListUpdatedNotification struct {
	Data []AppListUpdatedNotificationDatum `json:"data"`
}

// EXPERIMENTAL - app metadata returned by app-list APIs.
type AppListUpdatedNotificationDatum struct {
	AppMetadata                                                                             *PurpleAppMetadata `json:"appMetadata"`
	Branding                                                                                *PurpleAppBranding `json:"branding"`
	Description                                                                             *string            `json:"description"`
	DistributionChannel                                                                     *string            `json:"distributionChannel"`
	ID                                                                                      string             `json:"id"`
	InstallURL                                                                              *string            `json:"installUrl"`
	IsAccessible                                                                            *bool              `json:"isAccessible,omitempty"`
	// Whether this app is enabled in config.toml. Example: ```toml [apps.bad_app] enabled =                   
	// false ```                                                                                               
	IsEnabled                                                                               *bool              `json:"isEnabled,omitempty"`
	Labels                                                                                  map[string]string  `json:"labels"`
	LogoURL                                                                                 *string            `json:"logoUrl"`
	LogoURLDark                                                                             *string            `json:"logoUrlDark"`
	Name                                                                                    string             `json:"name"`
	PluginDisplayNames                                                                      []string           `json:"pluginDisplayNames,omitempty"`
}

type PurpleAppMetadata struct {
	Categories                 []string              `json:"categories"`
	Developer                  *string               `json:"developer"`
	FirstPartyRequiresInstall  *bool                 `json:"firstPartyRequiresInstall"`
	FirstPartyType             *string               `json:"firstPartyType"`
	Review                     *PurpleAppReview      `json:"review"`
	Screenshots                []PurpleAppScreenshot `json:"screenshots"`
	SEODescription             *string               `json:"seoDescription"`
	ShowInComposerWhenUnlinked *bool                 `json:"showInComposerWhenUnlinked"`
	SubCategories              []string              `json:"subCategories"`
	Version                    *string               `json:"version"`
	VersionID                  *string               `json:"versionId"`
	VersionNotes               *string               `json:"versionNotes"`
}

type PurpleAppReview struct {
	Status string `json:"status"`
}

type PurpleAppScreenshot struct {
	FileID     *string `json:"fileId"`
	URL        *string `json:"url"`
	UserPrompt string  `json:"userPrompt"`
}

// EXPERIMENTAL - app metadata returned by app-list APIs.
type PurpleAppBranding struct {
	Category          *string `json:"category"`
	Developer         *string `json:"developer"`
	IsDiscoverableApp bool    `json:"isDiscoverableApp"`
	PrivacyPolicy     *string `json:"privacyPolicy"`
	TermsOfService    *string `json:"termsOfService"`
	Website           *string `json:"website"`
}

// EXPERIMENTAL - list available apps/connectors.
type AppsListParams struct {
	// Opaque pagination cursor returned by a previous call.                                    
	Cursor                                                                              *string `json:"cursor"`
	// When true, bypass app caches and fetch the latest data from sources.                     
	ForceRefetch                                                                        *bool   `json:"forceRefetch,omitempty"`
	// Optional page size; defaults to a reasonable server-side value.                          
	Limit                                                                               *int64  `json:"limit"`
	// Optional thread id used to evaluate app feature gating from that thread's config.        
	ThreadID                                                                            *string `json:"threadId"`
}

// EXPERIMENTAL - app list response.
type AppsListResponse struct {
	Data                                                                                     []AppsListResponseDatum `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. If None, there                        
	// are no more items to return.                                                                                  
	NextCursor                                                                               *string                 `json:"nextCursor"`
}

// EXPERIMENTAL - app metadata returned by app-list APIs.
type AppsListResponseDatum struct {
	AppMetadata                                                                             *FluffyAppMetadata `json:"appMetadata"`
	Branding                                                                                *FluffyAppBranding `json:"branding"`
	Description                                                                             *string            `json:"description"`
	DistributionChannel                                                                     *string            `json:"distributionChannel"`
	ID                                                                                      string             `json:"id"`
	InstallURL                                                                              *string            `json:"installUrl"`
	IsAccessible                                                                            *bool              `json:"isAccessible,omitempty"`
	// Whether this app is enabled in config.toml. Example: ```toml [apps.bad_app] enabled =                   
	// false ```                                                                                               
	IsEnabled                                                                               *bool              `json:"isEnabled,omitempty"`
	Labels                                                                                  map[string]string  `json:"labels"`
	LogoURL                                                                                 *string            `json:"logoUrl"`
	LogoURLDark                                                                             *string            `json:"logoUrlDark"`
	Name                                                                                    string             `json:"name"`
	PluginDisplayNames                                                                      []string           `json:"pluginDisplayNames,omitempty"`
}

type FluffyAppMetadata struct {
	Categories                 []string              `json:"categories"`
	Developer                  *string               `json:"developer"`
	FirstPartyRequiresInstall  *bool                 `json:"firstPartyRequiresInstall"`
	FirstPartyType             *string               `json:"firstPartyType"`
	Review                     *FluffyAppReview      `json:"review"`
	Screenshots                []FluffyAppScreenshot `json:"screenshots"`
	SEODescription             *string               `json:"seoDescription"`
	ShowInComposerWhenUnlinked *bool                 `json:"showInComposerWhenUnlinked"`
	SubCategories              []string              `json:"subCategories"`
	Version                    *string               `json:"version"`
	VersionID                  *string               `json:"versionId"`
	VersionNotes               *string               `json:"versionNotes"`
}

type FluffyAppReview struct {
	Status string `json:"status"`
}

type FluffyAppScreenshot struct {
	FileID     *string `json:"fileId"`
	URL        *string `json:"url"`
	UserPrompt string  `json:"userPrompt"`
}

// EXPERIMENTAL - app metadata returned by app-list APIs.
type FluffyAppBranding struct {
	Category          *string `json:"category"`
	Developer         *string `json:"developer"`
	IsDiscoverableApp bool    `json:"isDiscoverableApp"`
	PrivacyPolicy     *string `json:"privacyPolicy"`
	TermsOfService    *string `json:"termsOfService"`
	Website           *string `json:"website"`
}

type CancelLoginAccountParams struct {
	LoginID string `json:"loginId"`
}

type CancelLoginAccountResponse struct {
	Status CancelLoginAccountStatus `json:"status"`
}

// Base64-encoded output chunk emitted for a streaming `command/exec` request.
//
// These notifications are connection-scoped. If the originating connection closes, the
// server terminates the process.
type CommandExecOutputDeltaNotification struct {
	// `true` on the final streamed chunk for a stream when `outputBytesCap` truncated later                
	// output on that stream.                                                                               
	CapReached                                                                                 bool         `json:"capReached"`
	// Base64-encoded output bytes.                                                                         
	DeltaBase64                                                                                string       `json:"deltaBase64"`
	// Client-supplied, connection-scoped `processId` from the original `command/exec` request.             
	ProcessID                                                                                  string       `json:"processId"`
	// Output stream for this chunk.                                                                        
	Stream                                                                                     OutputStream `json:"stream"`
}

// Run a standalone command (argv vector) in the server sandbox without creating a thread or
// turn.
//
// The final `command/exec` response is deferred until the process exits and is sent only
// after all `command/exec/outputDelta` notifications for that connection have been emitted.
type CommandExecParams struct {
	// Command argv vector. Empty arrays are rejected.                                                                                        
	Command                                                                                   []string                                        `json:"command"`
	// Optional working directory. Defaults to the server cwd.                                                                                
	Cwd                                                                                       *string                                         `json:"cwd"`
	// Disable stdout/stderr capture truncation for this request.                                                                             
	//                                                                                                                                        
	// Cannot be combined with `outputBytesCap`.                                                                                              
	DisableOutputCap                                                                          *bool                                           `json:"disableOutputCap,omitempty"`
	// Disable the timeout entirely for this request.                                                                                         
	//                                                                                                                                        
	// Cannot be combined with `timeoutMs`.                                                                                                   
	DisableTimeout                                                                            *bool                                           `json:"disableTimeout,omitempty"`
	// Optional environment overrides merged into the server-computed environment.                                                            
	//                                                                                                                                        
	// Matching names override inherited values. Set a key to `null` to unset an inherited                                                    
	// variable.                                                                                                                              
	Env                                                                                       map[string]*string                              `json:"env"`
	// Optional per-stream stdout/stderr capture cap in bytes.                                                                                
	//                                                                                                                                        
	// When omitted, the server default applies. Cannot be combined with `disableOutputCap`.                                                  
	OutputBytesCap                                                                            *int64                                          `json:"outputBytesCap"`
	// Optional client-supplied, connection-scoped process id.                                                                                
	//                                                                                                                                        
	// Required for `tty`, `streamStdin`, `streamStdoutStderr`, and follow-up                                                                 
	// `command/exec/write`, `command/exec/resize`, and `command/exec/terminate` calls. When                                                  
	// omitted, buffered execution gets an internal id that is not exposed to the client.                                                     
	ProcessID                                                                                 *string                                         `json:"processId"`
	// Optional sandbox policy for this command.                                                                                              
	//                                                                                                                                        
	// Uses the same shape as thread/turn execution sandbox configuration and defaults to the                                                 
	// user's configured policy when omitted. Cannot be combined with `permissionProfile`.                                                    
	SandboxPolicy                                                                             *CommandExecParamsDangerFullAccessSandboxPolicy `json:"sandboxPolicy"`
	// Optional initial PTY size in character cells. Only valid when `tty` is true.                                                           
	Size                                                                                      *CommandExecTerminalSize                        `json:"size"`
	// Allow follow-up `command/exec/write` requests to write stdin bytes.                                                                    
	//                                                                                                                                        
	// Requires a client-supplied `processId`.                                                                                                
	StreamStdin                                                                               *bool                                           `json:"streamStdin,omitempty"`
	// Stream stdout/stderr via `command/exec/outputDelta` notifications.                                                                     
	//                                                                                                                                        
	// Streamed bytes are not duplicated into the final response and require a client-supplied                                                
	// `processId`.                                                                                                                           
	StreamStdoutStderr                                                                        *bool                                           `json:"streamStdoutStderr,omitempty"`
	// Optional timeout in milliseconds.                                                                                                      
	//                                                                                                                                        
	// When omitted, the server default applies. Cannot be combined with `disableTimeout`.                                                    
	TimeoutMS                                                                                 *int64                                          `json:"timeoutMs"`
	// Enable PTY mode.                                                                                                                       
	//                                                                                                                                        
	// This implies `streamStdin` and `streamStdoutStderr`.                                                                                   
	TTY                                                                                       *bool                                           `json:"tty,omitempty"`
}

type CommandExecParamsDangerFullAccessSandboxPolicy struct {
	Type                SandboxPolicyType   `json:"type"`
	NetworkAccess       *NetworkAccessUnion `json:"networkAccess"`
	ExcludeSlashTmp     *bool               `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool               `json:"excludeTmpdirEnvVar,omitempty"`
	WritableRoots       []string            `json:"writableRoots,omitempty"`
}

// PTY size in character cells for `command/exec` PTY sessions.
type CommandExecTerminalSize struct {
	// Terminal width in character cells.       
	Cols                                  int64 `json:"cols"`
	// Terminal height in character cells.      
	Rows                                  int64 `json:"rows"`
}

// Resize a running PTY-backed `command/exec` session.
type CommandExecResizeParams struct {
	// Client-supplied, connection-scoped `processId` from the original `command/exec` request.          
	ProcessID                                                                                  string    `json:"processId"`
	// New PTY size in character cells.                                                                  
	Size                                                                                       SizeClass `json:"size"`
}

// New PTY size in character cells.
//
// PTY size in character cells for `command/exec` PTY sessions.
type SizeClass struct {
	// Terminal width in character cells.       
	Cols                                  int64 `json:"cols"`
	// Terminal height in character cells.      
	Rows                                  int64 `json:"rows"`
}

// Final buffered result for `command/exec`.
type CommandExecResponse struct {
	// Process exit code.                                                   
	ExitCode                                                         int64  `json:"exitCode"`
	// Buffered stderr capture.                                             
	//                                                                      
	// Empty when stderr was streamed via `command/exec/outputDelta`.       
	Stderr                                                           string `json:"stderr"`
	// Buffered stdout capture.                                             
	//                                                                      
	// Empty when stdout was streamed via `command/exec/outputDelta`.       
	Stdout                                                           string `json:"stdout"`
}

// Terminate a running `command/exec` session.
type CommandExecTerminateParams struct {
	// Client-supplied, connection-scoped `processId` from the original `command/exec` request.       
	ProcessID                                                                                  string `json:"processId"`
}

// Write stdin bytes to a running `command/exec` session, close stdin, or both.
type CommandExecWriteParams struct {
	// Close stdin after writing `deltaBase64`, if present.                                            
	CloseStdin                                                                                 *bool   `json:"closeStdin,omitempty"`
	// Optional base64-encoded stdin bytes to write.                                                   
	DeltaBase64                                                                                *string `json:"deltaBase64"`
	// Client-supplied, connection-scoped `processId` from the original `command/exec` request.        
	ProcessID                                                                                  string  `json:"processId"`
}

type CommandExecutionOutputDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type ConfigBatchWriteParams struct {
	Edits                                                                                  []ConfigEdit `json:"edits"`
	ExpectedVersion                                                                        *string      `json:"expectedVersion"`
	// Path to the config file to write; defaults to the user's `config.toml` when omitted.             
	FilePath                                                                               *string      `json:"filePath"`
	// When true, hot-reload the updated user config into all loaded threads after writing.             
	ReloadUserConfig                                                                       *bool        `json:"reloadUserConfig,omitempty"`
}

type ConfigEdit struct {
	KeyPath       string        `json:"keyPath"`
	MergeStrategy MergeStrategy `json:"mergeStrategy"`
	Value         interface{}   `json:"value"`
}

type ConfigReadParams struct {
	// Optional working directory to resolve project config layers. If specified, return the           
	// effective config as seen from that directory (i.e., including any project layers between        
	// `cwd` and the project/repo root).                                                               
	Cwd                                                                                        *string `json:"cwd"`
	IncludeLayers                                                                              *bool   `json:"includeLayers,omitempty"`
}

type ConfigReadResponse struct {
	Config  ConfigClass            `json:"config"`
	Layers  []ConfigLayer          `json:"layers"`
	Origins map[string]OriginValue `json:"origins"`
}

type ConfigClass struct {
	Analytics                                                                        *AnalyticsConfig            `json:"analytics"`
	ApprovalPolicy                                                                   *ApprovalPolicyUnion        `json:"approval_policy"`
	// [UNSTABLE] Optional default for where approval requests are routed for review.                            
	ApprovalsReviewer                                                                *ApprovalsReviewer          `json:"approvals_reviewer"`
	CompactPrompt                                                                    *string                     `json:"compact_prompt"`
	Desktop                                                                          map[string]interface{}      `json:"desktop"`
	DeveloperInstructions                                                            *string                     `json:"developer_instructions"`
	ForcedChatgptWorkspaceID                                                         *ForcedChatgptWorkspaceIDS  `json:"forced_chatgpt_workspace_id"`
	ForcedLoginMethod                                                                *ForcedLoginMethod          `json:"forced_login_method"`
	Instructions                                                                     *string                     `json:"instructions"`
	Model                                                                            *string                     `json:"model"`
	ModelAutoCompactTokenLimit                                                       *int64                      `json:"model_auto_compact_token_limit"`
	ModelAutoCompactTokenLimitScope                                                  *AutoCompactTokenLimitScope `json:"model_auto_compact_token_limit_scope"`
	ModelContextWindow                                                               *int64                      `json:"model_context_window"`
	ModelProvider                                                                    *string                     `json:"model_provider"`
	ModelReasoningEffort                                                             *ReasoningEffort            `json:"model_reasoning_effort"`
	ModelReasoningSummary                                                            *ReasoningSummary           `json:"model_reasoning_summary"`
	ModelVerbosity                                                                   *Verbosity                  `json:"model_verbosity"`
	Profile                                                                          *string                     `json:"profile"`
	Profiles                                                                         map[string]ProfileV2        `json:"profiles,omitempty"`
	ReviewModel                                                                      *string                     `json:"review_model"`
	SandboxMode                                                                      *SandboxMode                `json:"sandbox_mode"`
	SandboxWorkspaceWrite                                                            *SandboxWorkspaceWrite      `json:"sandbox_workspace_write"`
	ServiceTier                                                                      *string                     `json:"service_tier"`
	Tools                                                                            *ToolsV2                    `json:"tools"`
	WebSearch                                                                        *WebSearchMode              `json:"web_search"`
}

type AnalyticsConfig struct {
	Enabled *bool `json:"enabled"`
}

type ApprovalPolicyGranularAskForApproval struct {
	Granular PurpleGranular `json:"granular"`
}

type PurpleGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

type ProfileV2 struct {
	ApprovalPolicy                                                                          *ApprovalPolicyUnion `json:"approval_policy"`
	// [UNSTABLE] Optional profile-level override for where approval requests are routed for                     
	// review. If omitted, the enclosing config default is used.                                                 
	ApprovalsReviewer                                                                       *ApprovalsReviewer   `json:"approvals_reviewer"`
	ChatgptBaseURL                                                                          *string              `json:"chatgpt_base_url"`
	Model                                                                                   *string              `json:"model"`
	ModelProvider                                                                           *string              `json:"model_provider"`
	ModelReasoningEffort                                                                    *ReasoningEffort     `json:"model_reasoning_effort"`
	ModelReasoningSummary                                                                   *ReasoningSummary    `json:"model_reasoning_summary"`
	ModelVerbosity                                                                          *Verbosity           `json:"model_verbosity"`
	ServiceTier                                                                             *string              `json:"service_tier"`
	Tools                                                                                   *ToolsV2             `json:"tools"`
	WebSearch                                                                               *WebSearchMode       `json:"web_search"`
}

type ToolsV2 struct {
	WebSearch *WebSearchToolConfig `json:"web_search"`
}

type WebSearchToolConfig struct {
	AllowedDomains []string           `json:"allowed_domains"`
	ContextSize    *Verbosity         `json:"context_size"`
	Location       *WebSearchLocation `json:"location"`
}

type WebSearchLocation struct {
	City     *string `json:"city"`
	Country  *string `json:"country"`
	Region   *string `json:"region"`
	Timezone *string `json:"timezone"`
}

type SandboxWorkspaceWrite struct {
	ExcludeSlashTmp     *bool    `json:"exclude_slash_tmp,omitempty"`
	ExcludeTmpdirEnvVar *bool    `json:"exclude_tmpdir_env_var,omitempty"`
	NetworkAccess       *bool    `json:"network_access,omitempty"`
	WritableRoots       []string `json:"writable_roots,omitempty"`
}

type ConfigLayer struct {
	Config         interface{}            `json:"config"`
	DisabledReason *string                `json:"disabledReason"`
	Name           LayerConfigLayerSource `json:"name"`
	Version        string                 `json:"version"`
}

// Managed preferences layer delivered by MDM (macOS only).
//
// Managed config layer from a file (usually `managed_config.toml`).
//
// User config layer from $CODEX_HOME/config.toml. This layer is special in that it is
// expected to be: - writable by the user - generally outside the workspace directory
//
// Path to a .codex/ folder within a project. There could be multiple of these between `cwd`
// and the project/repo root.
//
// Session-layer overrides supplied via `-c`/`--config`.
//
// `managed_config.toml` was designed to be a config that was loaded as the last layer on
// top of everything else. This scheme did not quite work out as intended, but we keep this
// variant as a "best effort" while we phase out `managed_config.toml` in favor of
// `requirements.toml`.
type LayerConfigLayerSource struct {
	Domain                                                                                     *string               `json:"domain,omitempty"`
	Key                                                                                        *string               `json:"key,omitempty"`
	Type                                                                                       ConfigLayerSourceType `json:"type"`
	// This is the path to the system config.toml file, though it is not guaranteed to exist.                        
	//                                                                                                               
	// This is the path to the user's config.toml file, though it is not guaranteed to exist.                        
	File                                                                                       *string               `json:"file,omitempty"`
	// Name of the selected profile-v2 config layered on top of the base user config, when this                      
	// layer represents one.                                                                                         
	Profile                                                                                    *string               `json:"profile"`
	DotCodexFolder                                                                             *string               `json:"dotCodexFolder,omitempty"`
}

type OriginValue struct {
	Name    LayerConfigLayerSource `json:"name"`
	Version string                 `json:"version"`
}

type ConfigRequirementsReadResponse struct {
	// Null if no requirements are configured (e.g. no requirements.toml/MDM entries).                    
	Requirements                                                                      *ConfigRequirements `json:"requirements"`
}

type ConfigRequirements struct {
	AllowedApprovalPolicies []AskForApprovalElement  `json:"allowedApprovalPolicies"`
	AllowedPermissions      []string                 `json:"allowedPermissions"`
	AllowedSandboxModes     []SandboxMode            `json:"allowedSandboxModes"`
	AllowedWebSearchModes   []WebSearchMode          `json:"allowedWebSearchModes"`
	AllowManagedHooksOnly   *bool                    `json:"allowManagedHooksOnly"`
	ComputerUse             *ComputerUseRequirements `json:"computerUse"`
	EnforceResidency        *ResidencyRequirement    `json:"enforceResidency"`
	FeatureRequirements     map[string]bool          `json:"featureRequirements"`
}

type PurpleGranularAskForApproval struct {
	Granular FluffyGranular `json:"granular"`
}

type FluffyGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

type ComputerUseRequirements struct {
	AllowLockedComputerUse *bool `json:"allowLockedComputerUse"`
}

type ConfigValueWriteParams struct {
	ExpectedVersion                                                                        *string       `json:"expectedVersion"`
	// Path to the config file to write; defaults to the user's `config.toml` when omitted.              
	FilePath                                                                               *string       `json:"filePath"`
	KeyPath                                                                                string        `json:"keyPath"`
	MergeStrategy                                                                          MergeStrategy `json:"mergeStrategy"`
	Value                                                                                  interface{}   `json:"value"`
}

type ConfigWarningNotification struct {
	// Optional extra guidance or error details.                               
	Details                                                         *string    `json:"details"`
	// Optional path to the config file that triggered the warning.            
	Path                                                            *string    `json:"path"`
	// Optional range for the error location inside the config file.           
	Range                                                           *TextRange `json:"range"`
	// Concise summary of the warning.                                         
	Summary                                                         string     `json:"summary"`
}

type TextRange struct {
	End   TextPosition `json:"end"`
	Start TextPosition `json:"start"`
}

type TextPosition struct {
	// 1-based column number (in Unicode scalar values).      
	Column                                              int64 `json:"column"`
	// 1-based line number.                                   
	Line                                                int64 `json:"line"`
}

type ConfigWriteResponse struct {
	// Canonical path to the config file that was written.                    
	FilePath                                              string              `json:"filePath"`
	OverriddenMetadata                                    *OverriddenMetadata `json:"overriddenMetadata"`
	Status                                                WriteStatus         `json:"status"`
	Version                                               string              `json:"version"`
}

type OverriddenMetadata struct {
	EffectiveValue  interface{}          `json:"effectiveValue"`
	Message         string               `json:"message"`
	OverridingLayer OverridingLayerClass `json:"overridingLayer"`
}

type OverridingLayerClass struct {
	Name    OverridingLayerConfigLayerSource `json:"name"`
	Version string                           `json:"version"`
}

// Managed preferences layer delivered by MDM (macOS only).
//
// Managed config layer from a file (usually `managed_config.toml`).
//
// User config layer from $CODEX_HOME/config.toml. This layer is special in that it is
// expected to be: - writable by the user - generally outside the workspace directory
//
// Path to a .codex/ folder within a project. There could be multiple of these between `cwd`
// and the project/repo root.
//
// Session-layer overrides supplied via `-c`/`--config`.
//
// `managed_config.toml` was designed to be a config that was loaded as the last layer on
// top of everything else. This scheme did not quite work out as intended, but we keep this
// variant as a "best effort" while we phase out `managed_config.toml` in favor of
// `requirements.toml`.
type OverridingLayerConfigLayerSource struct {
	Domain                                                                                     *string               `json:"domain,omitempty"`
	Key                                                                                        *string               `json:"key,omitempty"`
	Type                                                                                       ConfigLayerSourceType `json:"type"`
	// This is the path to the system config.toml file, though it is not guaranteed to exist.                        
	//                                                                                                               
	// This is the path to the user's config.toml file, though it is not guaranteed to exist.                        
	File                                                                                       *string               `json:"file,omitempty"`
	// Name of the selected profile-v2 config layered on top of the base user config, when this                      
	// layer represents one.                                                                                         
	Profile                                                                                    *string               `json:"profile"`
	DotCodexFolder                                                                             *string               `json:"dotCodexFolder,omitempty"`
}

// Deprecated: Use `ContextCompaction` item type instead.
type ContextCompactedNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type DeprecationNoticeNotification struct {
	// Optional extra guidance, such as migration steps or rationale.        
	Details                                                          *string `json:"details"`
	// Concise summary of what is deprecated.                                
	Summary                                                          string  `json:"summary"`
}

type ErrorNotification struct {
	Error     TurnError `json:"error"`
	ThreadID  string    `json:"threadId"`
	TurnID    string    `json:"turnId"`
	WillRetry bool      `json:"willRetry"`
}

type TurnError struct {
	AdditionalDetails *string              `json:"additionalDetails"`
	CodexErrorInfo    *ErrorCodexErrorInfo `json:"codexErrorInfo"`
	Message           string               `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type PurpleCodexErrorInfo struct {
	HTTPConnectionFailed           *PurpleHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *PurpleResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *PurpleResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *PurpleResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *PurpleActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type PurpleActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type PurpleHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type PurpleResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type PurpleResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type PurpleResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type ExperimentalFeatureEnablementSetParams struct {
	// Process-wide runtime feature enablement keyed by canonical feature name.                               
	//                                                                                                        
	// Only named features are updated. Omitted features are left unchanged. Send an empty map                
	// for a no-op.                                                                                           
	Enablement                                                                                map[string]bool `json:"enablement"`
}

type ExperimentalFeatureEnablementSetResponse struct {
	// Feature enablement entries updated by this request.                
	Enablement                                            map[string]bool `json:"enablement"`
}

type ExperimentalFeatureListParams struct {
	// Opaque pagination cursor returned by a previous call.                                            
	Cursor                                                                                      *string `json:"cursor"`
	// Optional page size; defaults to a reasonable server-side value.                                  
	Limit                                                                                       *int64  `json:"limit"`
	// Optional loaded thread id. Pass this when showing feature state for an existing thread so        
	// enablement is computed from that thread's refreshed config, including project-local              
	// config for the thread's cwd.                                                                     
	ThreadID                                                                                    *string `json:"threadId"`
}

type ExperimentalFeatureListResponse struct {
	Data                                                                                     []ExperimentalFeature `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. If None, there                      
	// are no more items to return.                                                                                
	NextCursor                                                                               *string               `json:"nextCursor"`
}

type ExperimentalFeature struct {
	// Announcement copy shown to users when the feature is introduced. Null when this feature                           
	// is not in beta.                                                                                                   
	Announcement                                                                                *string                  `json:"announcement"`
	// Whether this feature is enabled by default.                                                                       
	DefaultEnabled                                                                              bool                     `json:"defaultEnabled"`
	// Short summary describing what the feature does. Null when this feature is not in beta.                            
	Description                                                                                 *string                  `json:"description"`
	// User-facing display name shown in the experimental features UI. Null when this feature is                         
	// not in beta.                                                                                                      
	DisplayName                                                                                 *string                  `json:"displayName"`
	// Whether this feature is currently enabled in the loaded config.                                                   
	Enabled                                                                                     bool                     `json:"enabled"`
	// Stable key used in config.toml and CLI flag toggles.                                                              
	Name                                                                                        string                   `json:"name"`
	// Lifecycle stage of this feature flag.                                                                             
	Stage                                                                                       ExperimentalFeatureStage `json:"stage"`
}

type ExternalAgentConfigDetectParams struct {
	// Zero or more working directories to include for repo-scoped detection.                
	Cwds                                                                            []string `json:"cwds"`
	// If true, include detection under the user's home (~/.claude, ~/.codex, etc.).         
	IncludeHome                                                                     *bool    `json:"includeHome,omitempty"`
}

type ExternalAgentConfigDetectResponse struct {
	Items []ItemElement `json:"items"`
}

type ItemElement struct {
	// Null or empty means home-scoped migration; non-empty means repo-scoped migration.                                     
	Cwd                                                                                 *string                              `json:"cwd"`
	Description                                                                         string                               `json:"description"`
	Details                                                                             *ItemMigrationDetails                `json:"details"`
	ItemType                                                                            ExternalAgentConfigMigrationItemType `json:"itemType"`
}

type ItemMigrationDetails struct {
	Commands   []PurpleCommandMigration   `json:"commands,omitempty"`
	Hooks      []PurpleHookMigration      `json:"hooks,omitempty"`
	MCPServers []PurpleMCPServerMigration `json:"mcpServers,omitempty"`
	Plugins    []PurplePluginsMigration   `json:"plugins,omitempty"`
	Sessions   []PurpleSessionMigration   `json:"sessions,omitempty"`
	Subagents  []PurpleSubagentMigration  `json:"subagents,omitempty"`
}

type PurpleCommandMigration struct {
	Name string `json:"name"`
}

type PurpleHookMigration struct {
	Name string `json:"name"`
}

type PurpleMCPServerMigration struct {
	Name string `json:"name"`
}

type PurplePluginsMigration struct {
	MarketplaceName string   `json:"marketplaceName"`
	PluginNames     []string `json:"pluginNames"`
}

type PurpleSessionMigration struct {
	Cwd   string  `json:"cwd"`
	Path  string  `json:"path"`
	Title *string `json:"title"`
}

type PurpleSubagentMigration struct {
	Name string `json:"name"`
}

type ExternalAgentConfigImportParams struct {
	MigrationItems []MigrationItemElement `json:"migrationItems"`
}

type MigrationItemElement struct {
	// Null or empty means home-scoped migration; non-empty means repo-scoped migration.                                     
	Cwd                                                                                 *string                              `json:"cwd"`
	Description                                                                         string                               `json:"description"`
	Details                                                                             *MigrationItemMigrationDetails       `json:"details"`
	ItemType                                                                            ExternalAgentConfigMigrationItemType `json:"itemType"`
}

type MigrationItemMigrationDetails struct {
	Commands   []FluffyCommandMigration   `json:"commands,omitempty"`
	Hooks      []FluffyHookMigration      `json:"hooks,omitempty"`
	MCPServers []FluffyMCPServerMigration `json:"mcpServers,omitempty"`
	Plugins    []FluffyPluginsMigration   `json:"plugins,omitempty"`
	Sessions   []FluffySessionMigration   `json:"sessions,omitempty"`
	Subagents  []FluffySubagentMigration  `json:"subagents,omitempty"`
}

type FluffyCommandMigration struct {
	Name string `json:"name"`
}

type FluffyHookMigration struct {
	Name string `json:"name"`
}

type FluffyMCPServerMigration struct {
	Name string `json:"name"`
}

type FluffyPluginsMigration struct {
	MarketplaceName string   `json:"marketplaceName"`
	PluginNames     []string `json:"pluginNames"`
}

type FluffySessionMigration struct {
	Cwd   string  `json:"cwd"`
	Path  string  `json:"path"`
	Title *string `json:"title"`
}

type FluffySubagentMigration struct {
	Name string `json:"name"`
}

type FeedbackUploadParams struct {
	Classification string            `json:"classification"`
	ExtraLogFiles  []string          `json:"extraLogFiles"`
	IncludeLogs    bool              `json:"includeLogs"`
	Reason         *string           `json:"reason"`
	Tags           map[string]string `json:"tags"`
	ThreadID       *string           `json:"threadId"`
}

type FeedbackUploadResponse struct {
	ThreadID string `json:"threadId"`
}

// Deprecated legacy notification for `apply_patch` textual output.
//
// The server no longer emits this notification.
type FileChangeOutputDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type FileChangePatchUpdatedNotification struct {
	Changes  []FileChangePatchUpdatedNotificationChange `json:"changes"`
	ItemID   string                                     `json:"itemId"`
	ThreadID string                                     `json:"threadId"`
	TurnID   string                                     `json:"turnId"`
}

type FileChangePatchUpdatedNotificationChange struct {
	Diff string                `json:"diff"`
	Kind PurplePatchChangeKind `json:"kind"`
	Path string                `json:"path"`
}

type PurplePatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

// Filesystem watch notification emitted for `fs/watch` subscribers.
type FSChangedNotification struct {
	// File or directory paths associated with this event.         
	ChangedPaths                                          []string `json:"changedPaths"`
	// Watch identifier previously provided to `fs/watch`.         
	WatchID                                               string   `json:"watchId"`
}

// Copy a file or directory tree on the host filesystem.
type FSCopyParams struct {
	// Absolute destination path.                                    
	DestinationPath                                           string `json:"destinationPath"`
	// Required for directory copies; ignored for file copies.       
	Recursive                                                 *bool  `json:"recursive,omitempty"`
	// Absolute source path.                                         
	SourcePath                                                string `json:"sourcePath"`
}

// Create a directory on the host filesystem.
type FSCreateDirectoryParams struct {
	// Absolute directory path to create.                                           
	Path                                                                     string `json:"path"`
	// Whether parent directories should also be created. Defaults to `true`.       
	Recursive                                                                *bool  `json:"recursive"`
}

// Request metadata for an absolute path.
type FSGetMetadataParams struct {
	// Absolute path to inspect.       
	Path                        string `json:"path"`
}

// Metadata returned by `fs/getMetadata`.
type FSGetMetadataResponse struct {
	// File creation time in Unix milliseconds when available, otherwise `0`.          
	CreatedAtMS                                                                  int64 `json:"createdAtMs"`
	// Whether the path resolves to a directory.                                       
	IsDirectory                                                                  bool  `json:"isDirectory"`
	// Whether the path resolves to a regular file.                                    
	IsFile                                                                       bool  `json:"isFile"`
	// Whether the path itself is a symbolic link.                                     
	IsSymlink                                                                    bool  `json:"isSymlink"`
	// File modification time in Unix milliseconds when available, otherwise `0`.      
	ModifiedAtMS                                                                 int64 `json:"modifiedAtMs"`
}

// List direct child names for a directory.
type FSReadDirectoryParams struct {
	// Absolute directory path to read.       
	Path                               string `json:"path"`
}

// Directory entries returned by `fs/readDirectory`.
type FSReadDirectoryResponse struct {
	// Direct child entries in the requested directory.                       
	Entries                                            []FSReadDirectoryEntry `json:"entries"`
}

// A directory entry returned by `fs/readDirectory`.
type FSReadDirectoryEntry struct {
	// Direct child entry name only, not an absolute or relative path.       
	FileName                                                          string `json:"fileName"`
	// Whether this entry resolves to a directory.                           
	IsDirectory                                                       bool   `json:"isDirectory"`
	// Whether this entry resolves to a regular file.                        
	IsFile                                                            bool   `json:"isFile"`
}

// Read a file from the host filesystem.
type FSReadFileParams struct {
	// Absolute path to read.       
	Path                     string `json:"path"`
}

// Base64-encoded file contents returned by `fs/readFile`.
type FSReadFileResponse struct {
	// File contents encoded as base64.       
	DataBase64                         string `json:"dataBase64"`
}

// Remove a file or directory tree from the host filesystem.
type FSRemoveParams struct {
	// Whether missing paths should be ignored. Defaults to `true`.        
	Force                                                           *bool  `json:"force"`
	// Absolute path to remove.                                            
	Path                                                            string `json:"path"`
	// Whether directory removal should recurse. Defaults to `true`.       
	Recursive                                                       *bool  `json:"recursive"`
}

// Stop filesystem watch notifications for a prior `fs/watch`.
type FSUnwatchParams struct {
	// Watch identifier previously provided to `fs/watch`.       
	WatchID                                               string `json:"watchId"`
}

// Start filesystem watch notifications for an absolute path.
type FSWatchParams struct {
	// Absolute file or directory path to watch.                                        
	Path                                                                         string `json:"path"`
	// Connection-scoped watch identifier used for `fs/unwatch` and `fs/changed`.       
	WatchID                                                                      string `json:"watchId"`
}

// Successful response for `fs/watch`.
type FSWatchResponse struct {
	// Canonicalized path associated with the watch.       
	Path                                            string `json:"path"`
}

// Write a file on the host filesystem.
type FSWriteFileParams struct {
	// File contents encoded as base64.       
	DataBase64                         string `json:"dataBase64"`
	// Absolute path to write.                
	Path                               string `json:"path"`
}

type GetAccountParams struct {
	// When `true`, requests a proactive token refresh before returning.                            
	//                                                                                              
	// In managed auth mode this triggers the normal refresh-token flow. In external auth mode      
	// this flag is ignored. Clients should refresh tokens themselves and call                      
	// `account/login/start` with `chatgptAuthTokens`.                                              
	RefreshToken                                                                              *bool `json:"refreshToken,omitempty"`
}

type GetAccountRateLimitsResponse struct {
	// Backward-compatible single-bucket view; mirrors the historical payload.                                    
	RateLimits                                                                RateLimitsByLimitIDClass            `json:"rateLimits"`
	// Multi-bucket view keyed by metered `limit_id` (for example, `codex`).                                      
	RateLimitsByLimitID                                                       map[string]RateLimitsByLimitIDClass `json:"rateLimitsByLimitId"`
}

// Backward-compatible single-bucket view; mirrors the historical payload.
type RateLimitsByLimitIDClass struct {
	Credits              *RateLimitsByLimitIDCreditsSnapshot `json:"credits"`
	LimitID              *string                             `json:"limitId"`
	LimitName            *string                             `json:"limitName"`
	PlanType             *PlanType                           `json:"planType"`
	Primary              *RateLimitsByLimitIDRateLimitWindow `json:"primary"`
	RateLimitReachedType *RateLimitReachedType               `json:"rateLimitReachedType"`
	Secondary            *RateLimitsByLimitIDRateLimitWindow `json:"secondary"`
}

type RateLimitsByLimitIDCreditsSnapshot struct {
	Balance    *string `json:"balance"`
	HasCredits bool    `json:"hasCredits"`
	Unlimited  bool    `json:"unlimited"`
}

type RateLimitsByLimitIDRateLimitWindow struct {
	ResetsAt           *int64 `json:"resetsAt"`
	UsedPercent        int64  `json:"usedPercent"`
	WindowDurationMins *int64 `json:"windowDurationMins"`
}

type GetAccountResponse struct {
	Account            *Account `json:"account"`
	RequiresOpenaiAuth bool     `json:"requiresOpenaiAuth"`
}

type Account struct {
	Type     AccountType `json:"type"`
	Email    *string     `json:"email,omitempty"`
	PlanType *PlanType   `json:"planType,omitempty"`
}

type GuardianWarningNotification struct {
	// Concise guardian warning message for the user.       
	Message                                          string `json:"message"`
	// Thread target for the guardian warning.              
	ThreadID                                         string `json:"threadId"`
}

type HookCompletedNotification struct {
	Run      HookCompletedNotificationRun `json:"run"`
	ThreadID string                       `json:"threadId"`
	TurnID   *string                      `json:"turnId"`
}

type HookCompletedNotificationRun struct {
	CompletedAt   *int64                  `json:"completedAt"`
	DisplayOrder  int64                   `json:"displayOrder"`
	DurationMS    *int64                  `json:"durationMs"`
	Entries       []PurpleHookOutputEntry `json:"entries"`
	EventName     HookEventName           `json:"eventName"`
	ExecutionMode HookExecutionMode       `json:"executionMode"`
	HandlerType   HookHandlerType         `json:"handlerType"`
	ID            string                  `json:"id"`
	Scope         HookScope               `json:"scope"`
	Source        *HookSource             `json:"source,omitempty"`
	SourcePath    string                  `json:"sourcePath"`
	StartedAt     int64                   `json:"startedAt"`
	Status        HookRunStatus           `json:"status"`
	StatusMessage *string                 `json:"statusMessage"`
}

type PurpleHookOutputEntry struct {
	Kind HookOutputEntryKind `json:"kind"`
	Text string              `json:"text"`
}

type HookStartedNotification struct {
	Run      HookStartedNotificationRun `json:"run"`
	ThreadID string                     `json:"threadId"`
	TurnID   *string                    `json:"turnId"`
}

type HookStartedNotificationRun struct {
	CompletedAt   *int64                  `json:"completedAt"`
	DisplayOrder  int64                   `json:"displayOrder"`
	DurationMS    *int64                  `json:"durationMs"`
	Entries       []FluffyHookOutputEntry `json:"entries"`
	EventName     HookEventName           `json:"eventName"`
	ExecutionMode HookExecutionMode       `json:"executionMode"`
	HandlerType   HookHandlerType         `json:"handlerType"`
	ID            string                  `json:"id"`
	Scope         HookScope               `json:"scope"`
	Source        *HookSource             `json:"source,omitempty"`
	SourcePath    string                  `json:"sourcePath"`
	StartedAt     int64                   `json:"startedAt"`
	Status        HookRunStatus           `json:"status"`
	StatusMessage *string                 `json:"statusMessage"`
}

type FluffyHookOutputEntry struct {
	Kind HookOutputEntryKind `json:"kind"`
	Text string              `json:"text"`
}

type HooksListParams struct {
	// When empty, defaults to the current session working directory.         
	Cwds                                                             []string `json:"cwds,omitempty"`
}

type HooksListResponse struct {
	Data []HooksListEntry `json:"data"`
}

type HooksListEntry struct {
	Cwd      string          `json:"cwd"`
	Errors   []HookErrorInfo `json:"errors"`
	Hooks    []HookMetadata  `json:"hooks"`
	Warnings []string        `json:"warnings"`
}

type HookErrorInfo struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

type HookMetadata struct {
	Command       *string         `json:"command"`
	CurrentHash   string          `json:"currentHash"`
	DisplayOrder  int64           `json:"displayOrder"`
	Enabled       bool            `json:"enabled"`
	EventName     HookEventName   `json:"eventName"`
	HandlerType   HookHandlerType `json:"handlerType"`
	IsManaged     bool            `json:"isManaged"`
	Key           string          `json:"key"`
	Matcher       *string         `json:"matcher"`
	PluginID      *string         `json:"pluginId"`
	Source        HookSource      `json:"source"`
	SourcePath    string          `json:"sourcePath"`
	StatusMessage *string         `json:"statusMessage"`
	TimeoutSEC    int64           `json:"timeoutSec"`
	TrustStatus   HookTrustStatus `json:"trustStatus"`
}

type ItemCompletedNotification struct {
	// Unix timestamp (in milliseconds) when this item lifecycle completed.                                    
	CompletedAtMS                                                          int64                               `json:"completedAtMs"`
	Item                                                                   ItemCompletedNotificationThreadItem `json:"item"`
	ThreadID                                                               string                              `json:"threadId"`
	TurnID                                                                 string                              `json:"turnId"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type ItemCompletedNotificationThreadItem struct {
	Content                                                                                     []PurpleContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                      
	ID                                                                                          string                                   `json:"id"`
	Type                                                                                        ThreadItemType                           `json:"type"`
	Fragments                                                                                   []PurpleHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *PurpleMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                            `json:"phase"`
	Text                                                                                        *string                                  `json:"text,omitempty"`
	Summary                                                                                     []string                                 `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                          
	AggregatedOutput                                                                            *string                                  `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                       
	Command                                                                                     *string                                  `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                            
	// returns a list of CommandAction objects because a single shell command may be composed of                                         
	// many commands piped together.                                                                                                     
	CommandActions                                                                              []PurpleCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                  
	Cwd                                                                                         *string                                  `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                            
	//                                                                                                                                   
	// The duration of the MCP tool call in milliseconds.                                                                                
	//                                                                                                                                   
	// The duration of the dynamic tool call in milliseconds.                                                                            
	DurationMS                                                                                  *int64                                   `json:"durationMs"`
	// The command's exit code.                                                                                                          
	ExitCode                                                                                    *int64                                   `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                       
	ProcessID                                                                                   *string                                  `json:"processId"`
	Source                                                                                      *CommandExecutionSource                  `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                           
	Status                                                                                      *string                                  `json:"status,omitempty"`
	Changes                                                                                     []PurpleFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                              `json:"arguments"`
	Error                                                                                       *PurpleMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                  `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                  `json:"pluginId"`
	Result                                                                                      *PurpleResult                            `json:"result"`
	Server                                                                                      *string                                  `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                         
	Tool                                                                                        *string                                  `json:"tool,omitempty"`
	ContentItems                                                                                []PurpleDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                  `json:"namespace"`
	Success                                                                                     *bool                                    `json:"success"`
	// Last known status of the target agents, when available.                                                                           
	AgentsStates                                                                                map[string]PurpleCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                           
	Model                                                                                       *string                                  `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                 
	Prompt                                                                                      *string                                  `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                
	ReasoningEffort                                                                             *ReasoningEffort                         `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                               
	// corresponds to the newly spawned agent.                                                                                           
	ReceiverThreadIDS                                                                           []string                                 `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                
	SenderThreadID                                                                              *string                                  `json:"senderThreadId,omitempty"`
	Action                                                                                      *PurpleWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                  `json:"query,omitempty"`
	Path                                                                                        *string                                  `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                  `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                  `json:"savedPath"`
	Review                                                                                      *string                                  `json:"review,omitempty"`
}

type PurpleWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type PurpleCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type PurpleFileUpdateChange struct {
	Diff string                `json:"diff"`
	Kind FluffyPatchChangeKind `json:"kind"`
	Path string                `json:"path"`
}

type FluffyPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type PurpleCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type PurpleUserInput struct {
	Text                                                                         *string             `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                    
	TextElements                                                                 []PurpleTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType       `json:"type"`
	Detail                                                                       *ImageDetail        `json:"detail"`
	URL                                                                          *string             `json:"url,omitempty"`
	Path                                                                         *string             `json:"path,omitempty"`
	Name                                                                         *string             `json:"name,omitempty"`
}

type PurpleTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                       
	ByteRange                                                                   PurpleByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                
	Placeholder                                                                 *string         `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type PurpleByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type PurpleDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type PurpleMCPToolCallError struct {
	Message string `json:"message"`
}

type PurpleHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type PurpleMemoryCitation struct {
	Entries   []PurpleMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                    `json:"threadIds"`
}

type PurpleMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type PurpleMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

// [UNSTABLE] Temporary notification payload for approval auto-review. This shape is
// expected to change soon.
type ItemGuardianApprovalReviewCompletedNotification struct {
	Action                                                                                      ItemGuardianApprovalReviewCompletedNotificationGuardianApprovalReviewAction `json:"action"`
	// Unix timestamp (in milliseconds) when this review completed.                                                                                                         
	CompletedAtMS                                                                               int64                                                                       `json:"completedAtMs"`
	DecisionSource                                                                              AutoReviewDecisionSource                                                    `json:"decisionSource"`
	Review                                                                                      ItemGuardianApprovalReviewCompletedNotificationReview                       `json:"review"`
	// Stable identifier for this review.                                                                                                                                   
	ReviewID                                                                                    string                                                                      `json:"reviewId"`
	// Unix timestamp (in milliseconds) when this review started.                                                                                                           
	StartedAtMS                                                                                 int64                                                                       `json:"startedAtMs"`
	// Identifier for the reviewed item or tool call when one exists.                                                                                                       
	//                                                                                                                                                                      
	// In most cases, one review maps to one target item. The exceptions are - execve reviews,                                                                              
	// where a single command may contain multiple execve calls to review (only possible when                                                                               
	// using the shell_zsh_fork feature) - network policy reviews, where there is no target                                                                                 
	// item                                                                                                                                                                 
	//                                                                                                                                                                      
	// A network call is triggered by a CommandExecution item, so having a target_item_id set to                                                                            
	// the CommandExecution item would be misleading because the review is about the network                                                                                
	// call, not the command execution. Therefore, target_item_id is set to None for network                                                                                
	// policy reviews.                                                                                                                                                      
	TargetItemID                                                                                *string                                                                     `json:"targetItemId"`
	ThreadID                                                                                    string                                                                      `json:"threadId"`
	TurnID                                                                                      string                                                                      `json:"turnId"`
}

type ItemGuardianApprovalReviewCompletedNotificationGuardianApprovalReviewAction struct {
	Command       *string                          `json:"command,omitempty"`
	Cwd           *string                          `json:"cwd,omitempty"`
	Source        *GuardianCommandSource           `json:"source,omitempty"`
	Type          GuardianApprovalReviewActionType `json:"type"`
	Argv          []string                         `json:"argv,omitempty"`
	Program       *string                          `json:"program,omitempty"`
	Files         []string                         `json:"files,omitempty"`
	Host          *string                          `json:"host,omitempty"`
	Port          *int64                           `json:"port,omitempty"`
	Protocol      *NetworkApprovalProtocol         `json:"protocol,omitempty"`
	Target        *string                          `json:"target,omitempty"`
	ConnectorID   *string                          `json:"connectorId"`
	ConnectorName *string                          `json:"connectorName"`
	Server        *string                          `json:"server,omitempty"`
	ToolName      *string                          `json:"toolName,omitempty"`
	ToolTitle     *string                          `json:"toolTitle"`
	Permissions   *PurpleRequestPermissionProfile  `json:"permissions,omitempty"`
	Reason        *string                          `json:"reason"`
}

type PurpleRequestPermissionProfile struct {
	FileSystem *PurpleAdditionalFileSystemPermissions `json:"fileSystem"`
	Network    *PurpleAdditionalNetworkPermissions    `json:"network"`
}

type PurpleAdditionalFileSystemPermissions struct {
	Entries                                       []PurpleFileSystemSandboxEntry `json:"entries"`
	GlobScanMaxDepth                              *int64                         `json:"globScanMaxDepth"`
	// This will be removed in favor of `entries`.                               
	Read                                          []string                       `json:"read"`
	// This will be removed in favor of `entries`.                               
	Write                                         []string                       `json:"write"`
}

type PurpleFileSystemSandboxEntry struct {
	Access FileSystemAccessMode `json:"access"`
	Path   PurpleFileSystemPath `json:"path"`
}

type PurpleFileSystemPath struct {
	Path    *string                      `json:"path,omitempty"`
	Type    FileSystemPathType           `json:"type"`
	Pattern *string                      `json:"pattern,omitempty"`
	Value   *PurpleFileSystemSpecialPath `json:"value,omitempty"`
}

type PurpleFileSystemSpecialPath struct {
	Kind    Kind    `json:"kind"`
	Subpath *string `json:"subpath"`
	Path    *string `json:"path,omitempty"`
}

type PurpleAdditionalNetworkPermissions struct {
	Enabled *bool `json:"enabled"`
}

// [UNSTABLE] Temporary approval auto-review payload used by `item/autoApprovalReview/*`
// notifications. This shape is expected to change soon.
type ItemGuardianApprovalReviewCompletedNotificationReview struct {
	Rationale         *string                      `json:"rationale"`
	RiskLevel         *GuardianRiskLevel           `json:"riskLevel"`
	Status            GuardianApprovalReviewStatus `json:"status"`
	UserAuthorization *GuardianUserAuthorization   `json:"userAuthorization"`
}

// [UNSTABLE] Temporary notification payload for approval auto-review. This shape is
// expected to change soon.
type ItemGuardianApprovalReviewStartedNotification struct {
	Action                                                                                      ItemGuardianApprovalReviewStartedNotificationGuardianApprovalReviewAction `json:"action"`
	Review                                                                                      ItemGuardianApprovalReviewStartedNotificationReview                       `json:"review"`
	// Stable identifier for this review.                                                                                                                                 
	ReviewID                                                                                    string                                                                    `json:"reviewId"`
	// Unix timestamp (in milliseconds) when this review started.                                                                                                         
	StartedAtMS                                                                                 int64                                                                     `json:"startedAtMs"`
	// Identifier for the reviewed item or tool call when one exists.                                                                                                     
	//                                                                                                                                                                    
	// In most cases, one review maps to one target item. The exceptions are - execve reviews,                                                                            
	// where a single command may contain multiple execve calls to review (only possible when                                                                             
	// using the shell_zsh_fork feature) - network policy reviews, where there is no target                                                                               
	// item                                                                                                                                                               
	//                                                                                                                                                                    
	// A network call is triggered by a CommandExecution item, so having a target_item_id set to                                                                          
	// the CommandExecution item would be misleading because the review is about the network                                                                              
	// call, not the command execution. Therefore, target_item_id is set to None for network                                                                              
	// policy reviews.                                                                                                                                                    
	TargetItemID                                                                                *string                                                                   `json:"targetItemId"`
	ThreadID                                                                                    string                                                                    `json:"threadId"`
	TurnID                                                                                      string                                                                    `json:"turnId"`
}

type ItemGuardianApprovalReviewStartedNotificationGuardianApprovalReviewAction struct {
	Command       *string                          `json:"command,omitempty"`
	Cwd           *string                          `json:"cwd,omitempty"`
	Source        *GuardianCommandSource           `json:"source,omitempty"`
	Type          GuardianApprovalReviewActionType `json:"type"`
	Argv          []string                         `json:"argv,omitempty"`
	Program       *string                          `json:"program,omitempty"`
	Files         []string                         `json:"files,omitempty"`
	Host          *string                          `json:"host,omitempty"`
	Port          *int64                           `json:"port,omitempty"`
	Protocol      *NetworkApprovalProtocol         `json:"protocol,omitempty"`
	Target        *string                          `json:"target,omitempty"`
	ConnectorID   *string                          `json:"connectorId"`
	ConnectorName *string                          `json:"connectorName"`
	Server        *string                          `json:"server,omitempty"`
	ToolName      *string                          `json:"toolName,omitempty"`
	ToolTitle     *string                          `json:"toolTitle"`
	Permissions   *FluffyRequestPermissionProfile  `json:"permissions,omitempty"`
	Reason        *string                          `json:"reason"`
}

type FluffyRequestPermissionProfile struct {
	FileSystem *FluffyAdditionalFileSystemPermissions `json:"fileSystem"`
	Network    *FluffyAdditionalNetworkPermissions    `json:"network"`
}

type FluffyAdditionalFileSystemPermissions struct {
	Entries                                       []FluffyFileSystemSandboxEntry `json:"entries"`
	GlobScanMaxDepth                              *int64                         `json:"globScanMaxDepth"`
	// This will be removed in favor of `entries`.                               
	Read                                          []string                       `json:"read"`
	// This will be removed in favor of `entries`.                               
	Write                                         []string                       `json:"write"`
}

type FluffyFileSystemSandboxEntry struct {
	Access FileSystemAccessMode `json:"access"`
	Path   FluffyFileSystemPath `json:"path"`
}

type FluffyFileSystemPath struct {
	Path    *string                      `json:"path,omitempty"`
	Type    FileSystemPathType           `json:"type"`
	Pattern *string                      `json:"pattern,omitempty"`
	Value   *FluffyFileSystemSpecialPath `json:"value,omitempty"`
}

type FluffyFileSystemSpecialPath struct {
	Kind    Kind    `json:"kind"`
	Subpath *string `json:"subpath"`
	Path    *string `json:"path,omitempty"`
}

type FluffyAdditionalNetworkPermissions struct {
	Enabled *bool `json:"enabled"`
}

// [UNSTABLE] Temporary approval auto-review payload used by `item/autoApprovalReview/*`
// notifications. This shape is expected to change soon.
type ItemGuardianApprovalReviewStartedNotificationReview struct {
	Rationale         *string                      `json:"rationale"`
	RiskLevel         *GuardianRiskLevel           `json:"riskLevel"`
	Status            GuardianApprovalReviewStatus `json:"status"`
	UserAuthorization *GuardianUserAuthorization   `json:"userAuthorization"`
}

type ItemStartedNotification struct {
	Item                                                                 ItemStartedNotificationThreadItem `json:"item"`
	// Unix timestamp (in milliseconds) when this item lifecycle started.                                  
	StartedAtMS                                                          int64                             `json:"startedAtMs"`
	ThreadID                                                             string                            `json:"threadId"`
	TurnID                                                               string                            `json:"turnId"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type ItemStartedNotificationThreadItem struct {
	Content                                                                                     []FluffyContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                      
	ID                                                                                          string                                   `json:"id"`
	Type                                                                                        ThreadItemType                           `json:"type"`
	Fragments                                                                                   []FluffyHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *FluffyMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                            `json:"phase"`
	Text                                                                                        *string                                  `json:"text,omitempty"`
	Summary                                                                                     []string                                 `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                          
	AggregatedOutput                                                                            *string                                  `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                       
	Command                                                                                     *string                                  `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                            
	// returns a list of CommandAction objects because a single shell command may be composed of                                         
	// many commands piped together.                                                                                                     
	CommandActions                                                                              []FluffyCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                  
	Cwd                                                                                         *string                                  `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                            
	//                                                                                                                                   
	// The duration of the MCP tool call in milliseconds.                                                                                
	//                                                                                                                                   
	// The duration of the dynamic tool call in milliseconds.                                                                            
	DurationMS                                                                                  *int64                                   `json:"durationMs"`
	// The command's exit code.                                                                                                          
	ExitCode                                                                                    *int64                                   `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                       
	ProcessID                                                                                   *string                                  `json:"processId"`
	Source                                                                                      *CommandExecutionSource                  `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                           
	Status                                                                                      *string                                  `json:"status,omitempty"`
	Changes                                                                                     []FluffyFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                              `json:"arguments"`
	Error                                                                                       *FluffyMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                  `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                  `json:"pluginId"`
	Result                                                                                      *FluffyResult                            `json:"result"`
	Server                                                                                      *string                                  `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                         
	Tool                                                                                        *string                                  `json:"tool,omitempty"`
	ContentItems                                                                                []FluffyDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                  `json:"namespace"`
	Success                                                                                     *bool                                    `json:"success"`
	// Last known status of the target agents, when available.                                                                           
	AgentsStates                                                                                map[string]FluffyCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                           
	Model                                                                                       *string                                  `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                 
	Prompt                                                                                      *string                                  `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                
	ReasoningEffort                                                                             *ReasoningEffort                         `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                               
	// corresponds to the newly spawned agent.                                                                                           
	ReceiverThreadIDS                                                                           []string                                 `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                
	SenderThreadID                                                                              *string                                  `json:"senderThreadId,omitempty"`
	Action                                                                                      *FluffyWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                  `json:"query,omitempty"`
	Path                                                                                        *string                                  `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                  `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                  `json:"savedPath"`
	Review                                                                                      *string                                  `json:"review,omitempty"`
}

type FluffyWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type FluffyCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type FluffyFileUpdateChange struct {
	Diff string                   `json:"diff"`
	Kind TentacledPatchChangeKind `json:"kind"`
	Path string                   `json:"path"`
}

type TentacledPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type FluffyCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type FluffyUserInput struct {
	Text                                                                         *string             `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                    
	TextElements                                                                 []FluffyTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType       `json:"type"`
	Detail                                                                       *ImageDetail        `json:"detail"`
	URL                                                                          *string             `json:"url,omitempty"`
	Path                                                                         *string             `json:"path,omitempty"`
	Name                                                                         *string             `json:"name,omitempty"`
}

type FluffyTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                       
	ByteRange                                                                   FluffyByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                
	Placeholder                                                                 *string         `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type FluffyByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type FluffyDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type FluffyMCPToolCallError struct {
	Message string `json:"message"`
}

type FluffyHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type FluffyMemoryCitation struct {
	Entries   []FluffyMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                    `json:"threadIds"`
}

type FluffyMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type FluffyMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ListMCPServerStatusParams struct {
	// Opaque pagination cursor returned by a previous call.                                                        
	Cursor                                                                                   *string                `json:"cursor"`
	// Controls how much MCP inventory data to fetch for each server. Defaults to `Full` when                       
	// omitted.                                                                                                     
	Detail                                                                                   *MCPServerStatusDetail `json:"detail"`
	// Optional page size; defaults to a server-defined value.                                                      
	Limit                                                                                    *int64                 `json:"limit"`
}

type ListMCPServerStatusResponse struct {
	Data                                                                                     []MCPServerStatus `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. If None, there                  
	// are no more items to return.                                                                            
	NextCursor                                                                               *string           `json:"nextCursor"`
}

type MCPServerStatus struct {
	AuthStatus        MCPAuthStatus      `json:"authStatus"`
	Name              string             `json:"name"`
	Resources         []Resource         `json:"resources"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	Tools             map[string]Tool    `json:"tools"`
}

// A template description for resources available on the server.
type ResourceTemplate struct {
	Annotations interface{} `json:"annotations"`
	Description *string     `json:"description"`
	MIMEType    *string     `json:"mimeType"`
	Name        string      `json:"name"`
	Title       *string     `json:"title"`
	URITemplate string      `json:"uriTemplate"`
}

// A known resource that the server is capable of reading.
type Resource struct {
	Meta        interface{}   `json:"_meta"`
	Annotations interface{}   `json:"annotations"`
	Description *string       `json:"description"`
	Icons       []interface{} `json:"icons"`
	MIMEType    *string       `json:"mimeType"`
	Name        string        `json:"name"`
	Size        *int64        `json:"size"`
	Title       *string       `json:"title"`
	URI         string        `json:"uri"`
}

// Definition for a tool the client can call.
type Tool struct {
	Meta         interface{}   `json:"_meta"`
	Annotations  interface{}   `json:"annotations"`
	Description  *string       `json:"description"`
	Icons        []interface{} `json:"icons"`
	InputSchema  interface{}   `json:"inputSchema"`
	Name         string        `json:"name"`
	OutputSchema interface{}   `json:"outputSchema"`
	Title        *string       `json:"title"`
}

// [UNSTABLE] FOR OPENAI INTERNAL USE ONLY - DO NOT USE. The access token must contain the
// same scopes that Codex-managed ChatGPT auth tokens have.
type LoginAccountParams struct {
	APIKey                                                                                   *string                `json:"apiKey,omitempty"`
	Type                                                                                     LoginAccountParamsType `json:"type"`
	CodexStreamlinedLogin                                                                    *bool                  `json:"codexStreamlinedLogin,omitempty"`
	// Access token (JWT) supplied by the client. This token is used for backend API requests                       
	// and email extraction.                                                                                        
	AccessToken                                                                              *string                `json:"accessToken,omitempty"`
	// Workspace/account identifier supplied by the client.                                                         
	ChatgptAccountID                                                                         *string                `json:"chatgptAccountId,omitempty"`
	// Optional plan type supplied by the client.                                                                   
	//                                                                                                              
	// When `null`, Codex attempts to derive the plan type from access-token claims. If                             
	// unavailable, the plan defaults to `unknown`.                                                                 
	ChatgptPlanType                                                                          *string                `json:"chatgptPlanType"`
}

type LoginAccountResponse struct {
	Type                                                                             LoginAccountParamsType `json:"type"`
	// URL the client should open in a browser to initiate the OAuth flow.                                  
	AuthURL                                                                          *string                `json:"authUrl,omitempty"`
	LoginID                                                                          *string                `json:"loginId,omitempty"`
	// One-time code the user must enter after signing in.                                                  
	UserCode                                                                         *string                `json:"userCode,omitempty"`
	// URL the client should open in a browser to complete device code authorization.                       
	VerificationURL                                                                  *string                `json:"verificationUrl,omitempty"`
}

type MarketplaceAddParams struct {
	RefName     *string  `json:"refName"`
	Source      string   `json:"source"`
	SparsePaths []string `json:"sparsePaths"`
}

type MarketplaceAddResponse struct {
	AlreadyAdded    bool   `json:"alreadyAdded"`
	InstalledRoot   string `json:"installedRoot"`
	MarketplaceName string `json:"marketplaceName"`
}

type MarketplaceRemoveParams struct {
	MarketplaceName string `json:"marketplaceName"`
}

type MarketplaceRemoveResponse struct {
	InstalledRoot   *string `json:"installedRoot"`
	MarketplaceName string  `json:"marketplaceName"`
}

type MarketplaceUpgradeParams struct {
	MarketplaceName *string `json:"marketplaceName"`
}

type MarketplaceUpgradeResponse struct {
	Errors               []MarketplaceUpgradeErrorInfo `json:"errors"`
	SelectedMarketplaces []string                      `json:"selectedMarketplaces"`
	UpgradedRoots        []string                      `json:"upgradedRoots"`
}

type MarketplaceUpgradeErrorInfo struct {
	MarketplaceName string `json:"marketplaceName"`
	Message         string `json:"message"`
}

type MCPResourceReadParams struct {
	Server   string  `json:"server"`
	ThreadID *string `json:"threadId"`
	URI      string  `json:"uri"`
}

type MCPResourceReadResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// Contents returned when reading a resource from an MCP server.
type ResourceContent struct {
	Meta                        interface{} `json:"_meta"`
	MIMEType                    *string     `json:"mimeType"`
	Text                        *string     `json:"text,omitempty"`
	// The URI of this resource.            
	URI                         string      `json:"uri"`
	Blob                        *string     `json:"blob,omitempty"`
}

type MCPServerOauthLoginCompletedNotification struct {
	Error   *string `json:"error"`
	Name    string  `json:"name"`
	Success bool    `json:"success"`
}

type MCPServerOauthLoginParams struct {
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	TimeoutSecs *int64   `json:"timeoutSecs"`
}

type MCPServerOauthLoginResponse struct {
	AuthorizationURL string `json:"authorizationUrl"`
}

type MCPServerStatusUpdatedNotification struct {
	Error  *string               `json:"error"`
	Name   string                `json:"name"`
	Status MCPServerStartupState `json:"status"`
}

type MCPServerToolCallParams struct {
	Meta      interface{} `json:"_meta"`
	Arguments interface{} `json:"arguments"`
	Server    string      `json:"server"`
	ThreadID  string      `json:"threadId"`
	Tool      string      `json:"tool"`
}

type MCPServerToolCallResponse struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	IsError           *bool         `json:"isError"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type MCPToolCallProgressNotification struct {
	ItemID   string `json:"itemId"`
	Message  string `json:"message"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type ModelListParams struct {
	// Opaque pagination cursor returned by a previous call.                          
	Cursor                                                                    *string `json:"cursor"`
	// When true, include models that are hidden from the default picker list.        
	IncludeHidden                                                             *bool   `json:"includeHidden"`
	// Optional page size; defaults to a reasonable server-side value.                
	Limit                                                                     *int64  `json:"limit"`
}

type ModelListResponse struct {
	Data                                                                                     []Model `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. If None, there        
	// are no more items to return.                                                                  
	NextCursor                                                                               *string `json:"nextCursor"`
}

type Model struct {
	// Deprecated: use `serviceTiers` instead.                                                        
	AdditionalSpeedTiers                                                      []string                `json:"additionalSpeedTiers,omitempty"`
	AvailabilityNux                                                           *ModelAvailabilityNux   `json:"availabilityNux"`
	DefaultReasoningEffort                                                    ReasoningEffort         `json:"defaultReasoningEffort"`
	// Catalog default service tier id for this model, when one is configured.                        
	DefaultServiceTier                                                        *string                 `json:"defaultServiceTier"`
	Description                                                               string                  `json:"description"`
	DisplayName                                                               string                  `json:"displayName"`
	Hidden                                                                    bool                    `json:"hidden"`
	ID                                                                        string                  `json:"id"`
	InputModalities                                                           []InputModality         `json:"inputModalities,omitempty"`
	IsDefault                                                                 bool                    `json:"isDefault"`
	Model                                                                     string                  `json:"model"`
	ServiceTiers                                                              []ModelServiceTier      `json:"serviceTiers,omitempty"`
	SupportedReasoningEfforts                                                 []ReasoningEffortOption `json:"supportedReasoningEfforts"`
	SupportsPersonality                                                       *bool                   `json:"supportsPersonality,omitempty"`
	Upgrade                                                                   *string                 `json:"upgrade"`
	UpgradeInfo                                                               *ModelUpgradeInfo       `json:"upgradeInfo"`
}

type ModelAvailabilityNux struct {
	Message string `json:"message"`
}

type ModelServiceTier struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
}

type ReasoningEffortOption struct {
	Description     string          `json:"description"`
	ReasoningEffort ReasoningEffort `json:"reasoningEffort"`
}

type ModelUpgradeInfo struct {
	MigrationMarkdown *string `json:"migrationMarkdown"`
	Model             string  `json:"model"`
	ModelLink         *string `json:"modelLink"`
	UpgradeCopy       *string `json:"upgradeCopy"`
}

type ModelProviderCapabilitiesReadResponse struct {
	ImageGeneration bool `json:"imageGeneration"`
	NamespaceTools  bool `json:"namespaceTools"`
	WebSearch       bool `json:"webSearch"`
}

type ModelReroutedNotification struct {
	FromModel string             `json:"fromModel"`
	Reason    ModelRerouteReason `json:"reason"`
	ThreadID  string             `json:"threadId"`
	ToModel   string             `json:"toModel"`
	TurnID    string             `json:"turnId"`
}

type ModelVerificationNotification struct {
	ThreadID      string              `json:"threadId"`
	TurnID        string              `json:"turnId"`
	Verifications []ModelVerification `json:"verifications"`
}

type PermissionProfileListParams struct {
	// Opaque pagination cursor returned by a previous call.               
	Cursor                                                         *string `json:"cursor"`
	// Optional working directory to resolve project config layers.        
	Cwd                                                            *string `json:"cwd"`
	// Optional page size; defaults to the full result set.                
	Limit                                                          *int64  `json:"limit"`
}

type PermissionProfileListResponse struct {
	Data                                                                                     []PermissionProfileSummary `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. If None, there                           
	// are no more items to return.                                                                                     
	NextCursor                                                                               *string                    `json:"nextCursor"`
}

type PermissionProfileSummary struct {
	// Optional user-facing description for display in clients.        
	Description                                                *string `json:"description"`
	// Available permission profile identifier.                        
	ID                                                         string  `json:"id"`
}

// EXPERIMENTAL - proposed plan streaming deltas for plan items. Clients should not assume
// concatenated deltas match the completed plan item content.
type PlanDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type PluginInstallParams struct {
	MarketplacePath       *string `json:"marketplacePath"`
	PluginName            string  `json:"pluginName"`
	RemoteMarketplaceName *string `json:"remoteMarketplaceName"`
}

type PluginInstallResponse struct {
	AppsNeedingAuth []AppsNeedingAuthElement `json:"appsNeedingAuth"`
	AuthPolicy      PluginAuthPolicy         `json:"authPolicy"`
}

// EXPERIMENTAL - app metadata summary for plugin responses.
type AppsNeedingAuthElement struct {
	Description *string `json:"description"`
	ID          string  `json:"id"`
	InstallURL  *string `json:"installUrl"`
	Name        string  `json:"name"`
	NeedsAuth   bool    `json:"needsAuth"`
}

type PluginInstalledParams struct {
	// Optional working directories used to discover repo marketplaces.                                  
	Cwds                                                                                        []string `json:"cwds"`
	// Additional uninstalled plugin names that should be returned when present locally. This is         
	// used by mention surfaces that intentionally expose install entrypoints.                           
	InstallSuggestionPluginNames                                                                []string `json:"installSuggestionPluginNames"`
}

type PluginInstalledResponse struct {
	MarketplaceLoadErrors []PluginInstalledResponseMarketplaceLoadError `json:"marketplaceLoadErrors,omitempty"`
	Marketplaces          []PluginInstalledResponseMarketplace          `json:"marketplaces"`
}

type PluginInstalledResponseMarketplaceLoadError struct {
	MarketplacePath string `json:"marketplacePath"`
	Message         string `json:"message"`
}

type PluginInstalledResponseMarketplace struct {
	Interface                                                                                 *PurpleMarketplaceInterface `json:"interface"`
	Name                                                                                      string                      `json:"name"`
	// Local marketplace file path when the marketplace is backed by a local file. Remote-only                            
	// catalog marketplaces do not have a local path.                                                                     
	Path                                                                                      *string                     `json:"path"`
	Plugins                                                                                   []PurplePluginSummary       `json:"plugins"`
}

type PurpleMarketplaceInterface struct {
	DisplayName *string `json:"displayName"`
}

type PurplePluginSummary struct {
	AuthPolicy                                                           PluginAuthPolicy          `json:"authPolicy"`
	// Availability state for installing and using the plugin.                                     
	Availability                                                         *PluginAvailability       `json:"availability,omitempty"`
	Enabled                                                              bool                      `json:"enabled"`
	ID                                                                   string                    `json:"id"`
	Installed                                                            bool                      `json:"installed"`
	InstallPolicy                                                        PluginInstallPolicy       `json:"installPolicy"`
	Interface                                                            *PurplePluginInterface    `json:"interface"`
	Keywords                                                             []string                  `json:"keywords,omitempty"`
	// Version of the locally materialized plugin package when available.                          
	LocalVersion                                                         *string                   `json:"localVersion"`
	Name                                                                 string                    `json:"name"`
	// Backend remote plugin identifier when available.                                            
	RemotePluginID                                                       *string                   `json:"remotePluginId"`
	// Remote sharing context associated with this plugin when available.                          
	ShareContext                                                         *PurplePluginShareContext `json:"shareContext"`
	Source                                                               PurplePluginSource        `json:"source"`
}

type PurplePluginInterface struct {
	BrandColor                                                                                 *string  `json:"brandColor"`
	Capabilities                                                                               []string `json:"capabilities"`
	Category                                                                                   *string  `json:"category"`
	// Local composer icon path, resolved from the installed plugin package.                            
	ComposerIcon                                                                               *string  `json:"composerIcon"`
	// Remote composer icon URL from the plugin catalog.                                                
	ComposerIconURL                                                                            *string  `json:"composerIconUrl"`
	// Starter prompts for the plugin. Capped at 3 entries with a maximum of 128 characters per         
	// entry.                                                                                           
	DefaultPrompt                                                                              []string `json:"defaultPrompt"`
	DeveloperName                                                                              *string  `json:"developerName"`
	DisplayName                                                                                *string  `json:"displayName"`
	// Local logo path, resolved from the installed plugin package.                                     
	Logo                                                                                       *string  `json:"logo"`
	// Remote logo URL from the plugin catalog.                                                         
	LogoURL                                                                                    *string  `json:"logoUrl"`
	LongDescription                                                                            *string  `json:"longDescription"`
	PrivacyPolicyURL                                                                           *string  `json:"privacyPolicyUrl"`
	// Local screenshot paths, resolved from the installed plugin package.                              
	Screenshots                                                                                []string `json:"screenshots"`
	// Remote screenshot URLs from the plugin catalog.                                                  
	ScreenshotUrls                                                                             []string `json:"screenshotUrls"`
	ShortDescription                                                                           *string  `json:"shortDescription"`
	TermsOfServiceURL                                                                          *string  `json:"termsOfServiceUrl"`
	WebsiteURL                                                                                 *string  `json:"websiteUrl"`
}

type PurplePluginShareContext struct {
	CreatorAccountUserID                                          *string                      `json:"creatorAccountUserId"`
	CreatorName                                                   *string                      `json:"creatorName"`
	Discoverability                                               *PluginShareDiscoverability  `json:"discoverability"`
	RemotePluginID                                                string                       `json:"remotePluginId"`
	// Version of the remote shared plugin release when available.                             
	RemoteVersion                                                 *string                      `json:"remoteVersion"`
	SharePrincipals                                               []PurplePluginSharePrincipal `json:"sharePrincipals"`
	ShareURL                                                      *string                      `json:"shareUrl"`
}

type PurplePluginSharePrincipal struct {
	Name          string                   `json:"name"`
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginSharePrincipalRole `json:"role"`
}

// The plugin is available in the remote catalog. Download metadata is kept server-side and
// is not exposed through the app-server API.
type PurplePluginSource struct {
	Path    *string          `json:"path"`
	Type    PluginSourceType `json:"type"`
	RefName *string          `json:"refName"`
	SHA     *string          `json:"sha"`
	URL     *string          `json:"url,omitempty"`
}

type PluginListParams struct {
	// Optional working directories used to discover repo marketplaces. When omitted, only                                  
	// home-scoped marketplaces and the official curated marketplace are considered.                                        
	Cwds                                                                                        []string                    `json:"cwds"`
	// Optional marketplace kind filter. When omitted, only local marketplaces are queried, plus                            
	// the default remote catalog when enabled by feature flag.                                                             
	MarketplaceKinds                                                                            []PluginListMarketplaceKind `json:"marketplaceKinds"`
}

type PluginListResponse struct {
	FeaturedPluginIDS     []string                                 `json:"featuredPluginIds,omitempty"`
	MarketplaceLoadErrors []PluginListResponseMarketplaceLoadError `json:"marketplaceLoadErrors,omitempty"`
	Marketplaces          []PluginListResponseMarketplace          `json:"marketplaces"`
}

type PluginListResponseMarketplaceLoadError struct {
	MarketplacePath string `json:"marketplacePath"`
	Message         string `json:"message"`
}

type PluginListResponseMarketplace struct {
	Interface                                                                                 *FluffyMarketplaceInterface `json:"interface"`
	Name                                                                                      string                      `json:"name"`
	// Local marketplace file path when the marketplace is backed by a local file. Remote-only                            
	// catalog marketplaces do not have a local path.                                                                     
	Path                                                                                      *string                     `json:"path"`
	Plugins                                                                                   []FluffyPluginSummary       `json:"plugins"`
}

type FluffyMarketplaceInterface struct {
	DisplayName *string `json:"displayName"`
}

type FluffyPluginSummary struct {
	AuthPolicy                                                           PluginAuthPolicy          `json:"authPolicy"`
	// Availability state for installing and using the plugin.                                     
	Availability                                                         *PluginAvailability       `json:"availability,omitempty"`
	Enabled                                                              bool                      `json:"enabled"`
	ID                                                                   string                    `json:"id"`
	Installed                                                            bool                      `json:"installed"`
	InstallPolicy                                                        PluginInstallPolicy       `json:"installPolicy"`
	Interface                                                            *FluffyPluginInterface    `json:"interface"`
	Keywords                                                             []string                  `json:"keywords,omitempty"`
	// Version of the locally materialized plugin package when available.                          
	LocalVersion                                                         *string                   `json:"localVersion"`
	Name                                                                 string                    `json:"name"`
	// Backend remote plugin identifier when available.                                            
	RemotePluginID                                                       *string                   `json:"remotePluginId"`
	// Remote sharing context associated with this plugin when available.                          
	ShareContext                                                         *FluffyPluginShareContext `json:"shareContext"`
	Source                                                               FluffyPluginSource        `json:"source"`
}

type FluffyPluginInterface struct {
	BrandColor                                                                                 *string  `json:"brandColor"`
	Capabilities                                                                               []string `json:"capabilities"`
	Category                                                                                   *string  `json:"category"`
	// Local composer icon path, resolved from the installed plugin package.                            
	ComposerIcon                                                                               *string  `json:"composerIcon"`
	// Remote composer icon URL from the plugin catalog.                                                
	ComposerIconURL                                                                            *string  `json:"composerIconUrl"`
	// Starter prompts for the plugin. Capped at 3 entries with a maximum of 128 characters per         
	// entry.                                                                                           
	DefaultPrompt                                                                              []string `json:"defaultPrompt"`
	DeveloperName                                                                              *string  `json:"developerName"`
	DisplayName                                                                                *string  `json:"displayName"`
	// Local logo path, resolved from the installed plugin package.                                     
	Logo                                                                                       *string  `json:"logo"`
	// Remote logo URL from the plugin catalog.                                                         
	LogoURL                                                                                    *string  `json:"logoUrl"`
	LongDescription                                                                            *string  `json:"longDescription"`
	PrivacyPolicyURL                                                                           *string  `json:"privacyPolicyUrl"`
	// Local screenshot paths, resolved from the installed plugin package.                              
	Screenshots                                                                                []string `json:"screenshots"`
	// Remote screenshot URLs from the plugin catalog.                                                  
	ScreenshotUrls                                                                             []string `json:"screenshotUrls"`
	ShortDescription                                                                           *string  `json:"shortDescription"`
	TermsOfServiceURL                                                                          *string  `json:"termsOfServiceUrl"`
	WebsiteURL                                                                                 *string  `json:"websiteUrl"`
}

type FluffyPluginShareContext struct {
	CreatorAccountUserID                                          *string                      `json:"creatorAccountUserId"`
	CreatorName                                                   *string                      `json:"creatorName"`
	Discoverability                                               *PluginShareDiscoverability  `json:"discoverability"`
	RemotePluginID                                                string                       `json:"remotePluginId"`
	// Version of the remote shared plugin release when available.                             
	RemoteVersion                                                 *string                      `json:"remoteVersion"`
	SharePrincipals                                               []FluffyPluginSharePrincipal `json:"sharePrincipals"`
	ShareURL                                                      *string                      `json:"shareUrl"`
}

type FluffyPluginSharePrincipal struct {
	Name          string                   `json:"name"`
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginSharePrincipalRole `json:"role"`
}

// The plugin is available in the remote catalog. Download metadata is kept server-side and
// is not exposed through the app-server API.
type FluffyPluginSource struct {
	Path    *string          `json:"path"`
	Type    PluginSourceType `json:"type"`
	RefName *string          `json:"refName"`
	SHA     *string          `json:"sha"`
	URL     *string          `json:"url,omitempty"`
}

type PluginReadParams struct {
	MarketplacePath       *string `json:"marketplacePath"`
	PluginName            string  `json:"pluginName"`
	RemoteMarketplaceName *string `json:"remoteMarketplaceName"`
}

type PluginReadResponse struct {
	Plugin PluginDetail `json:"plugin"`
}

type PluginDetail struct {
	Apps            []AppElement        `json:"apps"`
	Description     *string             `json:"description"`
	Hooks           []PluginHookSummary `json:"hooks"`
	MarketplaceName string              `json:"marketplaceName"`
	MarketplacePath *string             `json:"marketplacePath"`
	MCPServers      []string            `json:"mcpServers"`
	Skills          []SkillSummary      `json:"skills"`
	Summary         SummaryClass        `json:"summary"`
}

// EXPERIMENTAL - app metadata summary for plugin responses.
type AppElement struct {
	Description *string `json:"description"`
	ID          string  `json:"id"`
	InstallURL  *string `json:"installUrl"`
	Name        string  `json:"name"`
	NeedsAuth   bool    `json:"needsAuth"`
}

type PluginHookSummary struct {
	EventName HookEventName `json:"eventName"`
	Key       string        `json:"key"`
}

type SkillSummary struct {
	Description      string                `json:"description"`
	Enabled          bool                  `json:"enabled"`
	Interface        *PurpleSkillInterface `json:"interface"`
	Name             string                `json:"name"`
	Path             *string               `json:"path"`
	ShortDescription *string               `json:"shortDescription"`
}

type PurpleSkillInterface struct {
	BrandColor       *string `json:"brandColor"`
	DefaultPrompt    *string `json:"defaultPrompt"`
	DisplayName      *string `json:"displayName"`
	IconLarge        *string `json:"iconLarge"`
	IconSmall        *string `json:"iconSmall"`
	ShortDescription *string `json:"shortDescription"`
}

type SummaryClass struct {
	AuthPolicy                                                           PluginAuthPolicy           `json:"authPolicy"`
	// Availability state for installing and using the plugin.                                      
	Availability                                                         *PluginAvailability        `json:"availability,omitempty"`
	Enabled                                                              bool                       `json:"enabled"`
	ID                                                                   string                     `json:"id"`
	Installed                                                            bool                       `json:"installed"`
	InstallPolicy                                                        PluginInstallPolicy        `json:"installPolicy"`
	Interface                                                            *SummaryPluginInterface    `json:"interface"`
	Keywords                                                             []string                   `json:"keywords,omitempty"`
	// Version of the locally materialized plugin package when available.                           
	LocalVersion                                                         *string                    `json:"localVersion"`
	Name                                                                 string                     `json:"name"`
	// Backend remote plugin identifier when available.                                             
	RemotePluginID                                                       *string                    `json:"remotePluginId"`
	// Remote sharing context associated with this plugin when available.                           
	ShareContext                                                         *SummaryPluginShareContext `json:"shareContext"`
	Source                                                               SummaryPluginSource        `json:"source"`
}

type SummaryPluginInterface struct {
	BrandColor                                                                                 *string  `json:"brandColor"`
	Capabilities                                                                               []string `json:"capabilities"`
	Category                                                                                   *string  `json:"category"`
	// Local composer icon path, resolved from the installed plugin package.                            
	ComposerIcon                                                                               *string  `json:"composerIcon"`
	// Remote composer icon URL from the plugin catalog.                                                
	ComposerIconURL                                                                            *string  `json:"composerIconUrl"`
	// Starter prompts for the plugin. Capped at 3 entries with a maximum of 128 characters per         
	// entry.                                                                                           
	DefaultPrompt                                                                              []string `json:"defaultPrompt"`
	DeveloperName                                                                              *string  `json:"developerName"`
	DisplayName                                                                                *string  `json:"displayName"`
	// Local logo path, resolved from the installed plugin package.                                     
	Logo                                                                                       *string  `json:"logo"`
	// Remote logo URL from the plugin catalog.                                                         
	LogoURL                                                                                    *string  `json:"logoUrl"`
	LongDescription                                                                            *string  `json:"longDescription"`
	PrivacyPolicyURL                                                                           *string  `json:"privacyPolicyUrl"`
	// Local screenshot paths, resolved from the installed plugin package.                              
	Screenshots                                                                                []string `json:"screenshots"`
	// Remote screenshot URLs from the plugin catalog.                                                  
	ScreenshotUrls                                                                             []string `json:"screenshotUrls"`
	ShortDescription                                                                           *string  `json:"shortDescription"`
	TermsOfServiceURL                                                                          *string  `json:"termsOfServiceUrl"`
	WebsiteURL                                                                                 *string  `json:"websiteUrl"`
}

type SummaryPluginShareContext struct {
	CreatorAccountUserID                                          *string                         `json:"creatorAccountUserId"`
	CreatorName                                                   *string                         `json:"creatorName"`
	Discoverability                                               *PluginShareDiscoverability     `json:"discoverability"`
	RemotePluginID                                                string                          `json:"remotePluginId"`
	// Version of the remote shared plugin release when available.                                
	RemoteVersion                                                 *string                         `json:"remoteVersion"`
	SharePrincipals                                               []TentacledPluginSharePrincipal `json:"sharePrincipals"`
	ShareURL                                                      *string                         `json:"shareUrl"`
}

type TentacledPluginSharePrincipal struct {
	Name          string                   `json:"name"`
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginSharePrincipalRole `json:"role"`
}

// The plugin is available in the remote catalog. Download metadata is kept server-side and
// is not exposed through the app-server API.
type SummaryPluginSource struct {
	Path    *string          `json:"path"`
	Type    PluginSourceType `json:"type"`
	RefName *string          `json:"refName"`
	SHA     *string          `json:"sha"`
	URL     *string          `json:"url,omitempty"`
}

type PluginShareCheckoutParams struct {
	RemotePluginID string `json:"remotePluginId"`
}

type PluginShareCheckoutResponse struct {
	MarketplaceName string  `json:"marketplaceName"`
	MarketplacePath string  `json:"marketplacePath"`
	PluginID        string  `json:"pluginId"`
	PluginName      string  `json:"pluginName"`
	PluginPath      string  `json:"pluginPath"`
	RemotePluginID  string  `json:"remotePluginId"`
	RemoteVersion   *string `json:"remoteVersion"`
}

type PluginShareDeleteParams struct {
	RemotePluginID string `json:"remotePluginId"`
}

type PluginShareListResponse struct {
	Data []PluginShareListItem `json:"data"`
}

type PluginShareListItem struct {
	LocalPluginPath *string     `json:"localPluginPath"`
	Plugin          DatumPlugin `json:"plugin"`
}

type DatumPlugin struct {
	AuthPolicy                                                           PluginAuthPolicy             `json:"authPolicy"`
	// Availability state for installing and using the plugin.                                        
	Availability                                                         *PluginAvailability          `json:"availability,omitempty"`
	Enabled                                                              bool                         `json:"enabled"`
	ID                                                                   string                       `json:"id"`
	Installed                                                            bool                         `json:"installed"`
	InstallPolicy                                                        PluginInstallPolicy          `json:"installPolicy"`
	Interface                                                            *TentacledPluginInterface    `json:"interface"`
	Keywords                                                             []string                     `json:"keywords,omitempty"`
	// Version of the locally materialized plugin package when available.                             
	LocalVersion                                                         *string                      `json:"localVersion"`
	Name                                                                 string                       `json:"name"`
	// Backend remote plugin identifier when available.                                               
	RemotePluginID                                                       *string                      `json:"remotePluginId"`
	// Remote sharing context associated with this plugin when available.                             
	ShareContext                                                         *TentacledPluginShareContext `json:"shareContext"`
	Source                                                               TentacledPluginSource        `json:"source"`
}

type TentacledPluginInterface struct {
	BrandColor                                                                                 *string  `json:"brandColor"`
	Capabilities                                                                               []string `json:"capabilities"`
	Category                                                                                   *string  `json:"category"`
	// Local composer icon path, resolved from the installed plugin package.                            
	ComposerIcon                                                                               *string  `json:"composerIcon"`
	// Remote composer icon URL from the plugin catalog.                                                
	ComposerIconURL                                                                            *string  `json:"composerIconUrl"`
	// Starter prompts for the plugin. Capped at 3 entries with a maximum of 128 characters per         
	// entry.                                                                                           
	DefaultPrompt                                                                              []string `json:"defaultPrompt"`
	DeveloperName                                                                              *string  `json:"developerName"`
	DisplayName                                                                                *string  `json:"displayName"`
	// Local logo path, resolved from the installed plugin package.                                     
	Logo                                                                                       *string  `json:"logo"`
	// Remote logo URL from the plugin catalog.                                                         
	LogoURL                                                                                    *string  `json:"logoUrl"`
	LongDescription                                                                            *string  `json:"longDescription"`
	PrivacyPolicyURL                                                                           *string  `json:"privacyPolicyUrl"`
	// Local screenshot paths, resolved from the installed plugin package.                              
	Screenshots                                                                                []string `json:"screenshots"`
	// Remote screenshot URLs from the plugin catalog.                                                  
	ScreenshotUrls                                                                             []string `json:"screenshotUrls"`
	ShortDescription                                                                           *string  `json:"shortDescription"`
	TermsOfServiceURL                                                                          *string  `json:"termsOfServiceUrl"`
	WebsiteURL                                                                                 *string  `json:"websiteUrl"`
}

type TentacledPluginShareContext struct {
	CreatorAccountUserID                                          *string                      `json:"creatorAccountUserId"`
	CreatorName                                                   *string                      `json:"creatorName"`
	Discoverability                                               *PluginShareDiscoverability  `json:"discoverability"`
	RemotePluginID                                                string                       `json:"remotePluginId"`
	// Version of the remote shared plugin release when available.                             
	RemoteVersion                                                 *string                      `json:"remoteVersion"`
	SharePrincipals                                               []StickyPluginSharePrincipal `json:"sharePrincipals"`
	ShareURL                                                      *string                      `json:"shareUrl"`
}

type StickyPluginSharePrincipal struct {
	Name          string                   `json:"name"`
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginSharePrincipalRole `json:"role"`
}

// The plugin is available in the remote catalog. Download metadata is kept server-side and
// is not exposed through the app-server API.
type TentacledPluginSource struct {
	Path    *string          `json:"path"`
	Type    PluginSourceType `json:"type"`
	RefName *string          `json:"refName"`
	SHA     *string          `json:"sha"`
	URL     *string          `json:"url,omitempty"`
}

type PluginShareSaveParams struct {
	Discoverability *PluginShareDiscoverability        `json:"discoverability"`
	PluginPath      string                             `json:"pluginPath"`
	RemotePluginID  *string                            `json:"remotePluginId"`
	ShareTargets    []PluginShareSaveParamsShareTarget `json:"shareTargets"`
}

type PluginShareSaveParamsShareTarget struct {
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginShareTargetRole    `json:"role"`
}

type PluginShareSaveResponse struct {
	RemotePluginID string `json:"remotePluginId"`
	ShareURL       string `json:"shareUrl"`
}

type PluginShareUpdateTargetsParams struct {
	Discoverability PluginShareUpdateDiscoverability            `json:"discoverability"`
	RemotePluginID  string                                      `json:"remotePluginId"`
	ShareTargets    []PluginShareUpdateTargetsParamsShareTarget `json:"shareTargets"`
}

type PluginShareUpdateTargetsParamsShareTarget struct {
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginShareTargetRole    `json:"role"`
}

type PluginShareUpdateTargetsResponse struct {
	Discoverability PluginShareDiscoverability `json:"discoverability"`
	Principals      []PrincipalElement         `json:"principals"`
}

type PrincipalElement struct {
	Name          string                   `json:"name"`
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginSharePrincipalRole `json:"role"`
}

type PluginSkillReadParams struct {
	RemoteMarketplaceName string `json:"remoteMarketplaceName"`
	RemotePluginID        string `json:"remotePluginId"`
	SkillName             string `json:"skillName"`
}

type PluginSkillReadResponse struct {
	Contents *string `json:"contents"`
}

type PluginUninstallParams struct {
	PluginID string `json:"pluginId"`
}

// Final process exit notification for `process/spawn`.
type ProcessExitedNotification struct {
	// Process exit code.                                                                          
	ExitCode                                                                                int64  `json:"exitCode"`
	// Client-supplied, connection-scoped `processHandle` from `process/spawn`.                    
	ProcessHandle                                                                           string `json:"processHandle"`
	// Buffered stderr capture.                                                                    
	//                                                                                             
	// Empty when stderr was streamed via `process/outputDelta`.                                   
	Stderr                                                                                  string `json:"stderr"`
	// Whether stderr reached `outputBytesCap`.                                                    
	//                                                                                             
	// In streaming mode, stderr is empty and cap state is also reported on the final stderr       
	// `process/outputDelta` notification.                                                         
	StderrCapReached                                                                        bool   `json:"stderrCapReached"`
	// Buffered stdout capture.                                                                    
	//                                                                                             
	// Empty when stdout was streamed via `process/outputDelta`.                                   
	Stdout                                                                                  string `json:"stdout"`
	// Whether stdout reached `outputBytesCap`.                                                    
	//                                                                                             
	// In streaming mode, stdout is empty and cap state is also reported on the final stdout       
	// `process/outputDelta` notification.                                                         
	StdoutCapReached                                                                        bool   `json:"stdoutCapReached"`
}

// Base64-encoded output chunk emitted for a streaming `process/spawn` request.
type ProcessOutputDeltaNotification struct {
	// True on the final streamed chunk for this stream when output was truncated by             
	// `outputBytesCap`.                                                                         
	CapReached                                                                      bool         `json:"capReached"`
	// Base64-encoded output bytes.                                                              
	DeltaBase64                                                                     string       `json:"deltaBase64"`
	// Client-supplied, connection-scoped `processHandle` from `process/spawn`.                  
	ProcessHandle                                                                   string       `json:"processHandle"`
	// Output stream this chunk belongs to.                                                      
	Stream                                                                          OutputStream `json:"stream"`
}

type RawResponseItemCompletedNotification struct {
	Item     ResponseItem `json:"item"`
	ThreadID string       `json:"threadId"`
	TurnID   string       `json:"turnId"`
}

type ResponseItem struct {
	Content                                                           []ContentItem                   `json:"content"`
	// Legacy id field retained for compatibility with older payloads.                                
	ID                                                                *string                         `json:"id"`
	Phase                                                             *MessagePhase                   `json:"phase"`
	Role                                                              *string                         `json:"role,omitempty"`
	Type                                                              ResponseItemType                `json:"type"`
	EncryptedContent                                                  *string                         `json:"encrypted_content"`
	Summary                                                           []ReasoningItemReasoningSummary `json:"summary,omitempty"`
	Action                                                            *ResponsesAPIWebSearchAction    `json:"action"`
	// Set when using the Responses API.                                                              
	CallID                                                            *string                         `json:"call_id"`
	Status                                                            *string                         `json:"status"`
	Arguments                                                         interface{}                     `json:"arguments"`
	Name                                                              *string                         `json:"name"`
	Namespace                                                         *string                         `json:"namespace"`
	Execution                                                         *string                         `json:"execution,omitempty"`
	Output                                                            *FunctionCallOutputBody         `json:"output"`
	Input                                                             *string                         `json:"input,omitempty"`
	Tools                                                             []interface{}                   `json:"tools,omitempty"`
	Result                                                            *string                         `json:"result,omitempty"`
	RevisedPrompt                                                     *string                         `json:"revised_prompt"`
}

type ResponsesAPIWebSearchAction struct {
	Command          []string                 `json:"command,omitempty"`
	Env              map[string]string        `json:"env"`
	TimeoutMS        *int64                   `json:"timeout_ms"`
	Type             ExecLocalShellActionType `json:"type"`
	User             *string                  `json:"user"`
	WorkingDirectory *string                  `json:"working_directory"`
	Queries          []string                 `json:"queries"`
	Query            *string                  `json:"query"`
	URL              *string                  `json:"url"`
	Pattern          *string                  `json:"pattern"`
}

type ContentItem struct {
	Text     *string      `json:"text,omitempty"`
	Type     ContentType  `json:"type"`
	Detail   *ImageDetail `json:"detail"`
	ImageURL *string      `json:"image_url,omitempty"`
}

// Responses API compatible content items that can be returned by a tool call. This is a
// subset of ContentItem with the types we support as function call outputs.
type FunctionCallOutputContentItem struct {
	Text             *string                           `json:"text,omitempty"`
	Type             FunctionCallOutputContentItemType `json:"type"`
	Detail           *ImageDetail                      `json:"detail"`
	ImageURL         *string                           `json:"image_url,omitempty"`
	EncryptedContent *string                           `json:"encrypted_content,omitempty"`
}

type ReasoningItemReasoningSummary struct {
	Text string                                       `json:"text"`
	Type SummaryTextReasoningItemReasoningSummaryType `json:"type"`
}

type ReasoningSummaryPartAddedNotification struct {
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"`
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

type ReasoningSummaryTextDeltaNotification struct {
	Delta        string `json:"delta"`
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"`
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

type ReasoningTextDeltaNotification struct {
	ContentIndex int64  `json:"contentIndex"`
	Delta        string `json:"delta"`
	ItemID       string `json:"itemId"`
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// Current remote-control connection status and remote identity exposed to clients.
type RemoteControlStatusChangedNotification struct {
	EnvironmentID  *string                       `json:"environmentId"`
	InstallationID string                        `json:"installationId"`
	ServerName     string                        `json:"serverName"`
	Status         RemoteControlConnectionStatus `json:"status"`
}

type ReviewStartParams struct {
	// Where to run the review: inline (default) on the current thread or detached on a new                
	// thread (returned in `reviewThreadId`).                                                              
	Delivery                                                                               *ReviewDelivery `json:"delivery"`
	Target                                                                                 ReviewTarget    `json:"target"`
	ThreadID                                                                               string          `json:"threadId"`
}

// Review the working tree: staged, unstaged, and untracked files.
//
// Review changes between the current branch and the given base branch.
//
// Review the changes introduced by a specific commit.
//
// Arbitrary instructions, equivalent to the old free-form prompt.
type ReviewTarget struct {
	Type                                                            ReviewTargetType `json:"type"`
	Branch                                                          *string          `json:"branch,omitempty"`
	SHA                                                             *string          `json:"sha,omitempty"`
	// Optional human-readable label (e.g., commit subject) for UIs.                 
	Title                                                           *string          `json:"title"`
	Instructions                                                    *string          `json:"instructions,omitempty"`
}

type ReviewStartResponse struct {
	// Identifies the thread where the review runs.                                                                    
	//                                                                                                                 
	// For inline reviews, this is the original thread id. For detached reviews, this is the id                        
	// of the new review thread.                                                                                       
	ReviewThreadID                                                                             string                  `json:"reviewThreadId"`
	Turn                                                                                       ReviewStartResponseTurn `json:"turn"`
}

type ReviewStartResponseTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                    
	CompletedAt                                                             *int64             `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                   
	DurationMS                                                              *int64             `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                        
	Error                                                                   *PurpleTurnError   `json:"error"`
	ID                                                                      string             `json:"id"`
	// Thread items currently included in this turn payload.                                   
	Items                                                                   []PurpleThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                            
	ItemsView                                                               *TurnItemsView     `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                      
	StartedAt                                                               *int64             `json:"startedAt"`
	Status                                                                  TurnStatus         `json:"status"`
}

type PurpleTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo2 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type FluffyCodexErrorInfo struct {
	HTTPConnectionFailed           *FluffyHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *FluffyResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *FluffyResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *FluffyResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *FluffyActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type FluffyActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type FluffyHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type FluffyResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type FluffyResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type FluffyResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type PurpleThreadItem struct {
	Content                                                                                     []TentacledContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                         
	ID                                                                                          string                                      `json:"id"`
	Type                                                                                        ThreadItemType                              `json:"type"`
	Fragments                                                                                   []TentacledHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *TentacledMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                               `json:"phase"`
	Text                                                                                        *string                                     `json:"text,omitempty"`
	Summary                                                                                     []string                                    `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                             
	AggregatedOutput                                                                            *string                                     `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                          
	Command                                                                                     *string                                     `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                               
	// returns a list of CommandAction objects because a single shell command may be composed of                                            
	// many commands piped together.                                                                                                        
	CommandActions                                                                              []TentacledCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                     
	Cwd                                                                                         *string                                     `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                               
	//                                                                                                                                      
	// The duration of the MCP tool call in milliseconds.                                                                                   
	//                                                                                                                                      
	// The duration of the dynamic tool call in milliseconds.                                                                               
	DurationMS                                                                                  *int64                                      `json:"durationMs"`
	// The command's exit code.                                                                                                             
	ExitCode                                                                                    *int64                                      `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                          
	ProcessID                                                                                   *string                                     `json:"processId"`
	Source                                                                                      *CommandExecutionSource                     `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                              
	Status                                                                                      *string                                     `json:"status,omitempty"`
	Changes                                                                                     []TentacledFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                                 `json:"arguments"`
	Error                                                                                       *TentacledMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                     `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                     `json:"pluginId"`
	Result                                                                                      *TentacledResult                            `json:"result"`
	Server                                                                                      *string                                     `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                            
	Tool                                                                                        *string                                     `json:"tool,omitempty"`
	ContentItems                                                                                []TentacledDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                     `json:"namespace"`
	Success                                                                                     *bool                                       `json:"success"`
	// Last known status of the target agents, when available.                                                                              
	AgentsStates                                                                                map[string]TentacledCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                              
	Model                                                                                       *string                                     `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                    
	Prompt                                                                                      *string                                     `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                   
	ReasoningEffort                                                                             *ReasoningEffort                            `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                  
	// corresponds to the newly spawned agent.                                                                                              
	ReceiverThreadIDS                                                                           []string                                    `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                   
	SenderThreadID                                                                              *string                                     `json:"senderThreadId,omitempty"`
	Action                                                                                      *TentacledWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                     `json:"query,omitempty"`
	Path                                                                                        *string                                     `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                     `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                     `json:"savedPath"`
	Review                                                                                      *string                                     `json:"review,omitempty"`
}

type TentacledWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type TentacledCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type TentacledFileUpdateChange struct {
	Diff string                `json:"diff"`
	Kind StickyPatchChangeKind `json:"kind"`
	Path string                `json:"path"`
}

type StickyPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type TentacledCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type TentacledUserInput struct {
	Text                                                                         *string                `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                       
	TextElements                                                                 []TentacledTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType          `json:"type"`
	Detail                                                                       *ImageDetail           `json:"detail"`
	URL                                                                          *string                `json:"url,omitempty"`
	Path                                                                         *string                `json:"path,omitempty"`
	Name                                                                         *string                `json:"name,omitempty"`
}

type TentacledTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                          
	ByteRange                                                                   TentacledByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                   
	Placeholder                                                                 *string            `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type TentacledByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type TentacledDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type TentacledMCPToolCallError struct {
	Message string `json:"message"`
}

type TentacledHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type TentacledMemoryCitation struct {
	Entries   []TentacledMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                       `json:"threadIds"`
}

type TentacledMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type TentacledMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type SendAddCreditsNudgeEmailParams struct {
	CreditType AddCreditsNudgeCreditType `json:"creditType"`
}

type SendAddCreditsNudgeEmailResponse struct {
	Status AddCreditsNudgeEmailStatus `json:"status"`
}

type ServerRequestResolvedNotification struct {
	RequestID *RequestID `json:"requestId"`
	ThreadID  string     `json:"threadId"`
}

type SkillsConfigWriteParams struct {
	Enabled                bool    `json:"enabled"`
	// Name-based selector.        
	Name                   *string `json:"name"`
	// Path-based selector.        
	Path                   *string `json:"path"`
}

type SkillsConfigWriteResponse struct {
	EffectiveEnabled bool `json:"effectiveEnabled"`
}

type SkillsListParams struct {
	// When empty, defaults to the current session working directory.           
	Cwds                                                               []string `json:"cwds,omitempty"`
	// When true, bypass the skills cache and re-scan skills from disk.         
	ForceReload                                                        *bool    `json:"forceReload,omitempty"`
}

type SkillsListResponse struct {
	Data []SkillsListEntry `json:"data"`
}

type SkillsListEntry struct {
	Cwd    string           `json:"cwd"`
	Errors []SkillErrorInfo `json:"errors"`
	Skills []SkillMetadata  `json:"skills"`
}

type SkillErrorInfo struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

type SkillMetadata struct {
	Dependencies                                                                             *SkillDependencies    `json:"dependencies"`
	Description                                                                              string                `json:"description"`
	Enabled                                                                                  bool                  `json:"enabled"`
	Interface                                                                                *FluffySkillInterface `json:"interface"`
	Name                                                                                     string                `json:"name"`
	Path                                                                                     string                `json:"path"`
	Scope                                                                                    SkillScope            `json:"scope"`
	// Legacy short_description from SKILL.md. Prefer SKILL.json interface.short_description.                      
	ShortDescription                                                                         *string               `json:"shortDescription"`
}

type SkillDependencies struct {
	Tools []SkillToolDependency `json:"tools"`
}

type SkillToolDependency struct {
	Command     *string `json:"command"`
	Description *string `json:"description"`
	Transport   *string `json:"transport"`
	Type        string  `json:"type"`
	URL         *string `json:"url"`
	Value       string  `json:"value"`
}

type FluffySkillInterface struct {
	BrandColor       *string `json:"brandColor"`
	DefaultPrompt    *string `json:"defaultPrompt"`
	DisplayName      *string `json:"displayName"`
	IconLarge        *string `json:"iconLarge"`
	IconSmall        *string `json:"iconSmall"`
	ShortDescription *string `json:"shortDescription"`
}

type TerminalInteractionNotification struct {
	ItemID    string `json:"itemId"`
	ProcessID string `json:"processId"`
	Stdin     string `json:"stdin"`
	ThreadID  string `json:"threadId"`
	TurnID    string `json:"turnId"`
}

type ThreadApproveGuardianDeniedActionParams struct {
	// Serialized `codex_protocol::protocol::GuardianAssessmentEvent`.            
	Event                                                             interface{} `json:"event"`
	ThreadID                                                          string      `json:"threadId"`
}

type ThreadArchiveParams struct {
	ThreadID string `json:"threadId"`
}

type ThreadArchivedNotification struct {
	ThreadID string `json:"threadId"`
}

type ThreadClosedNotification struct {
	ThreadID string `json:"threadId"`
}

type ThreadCompactStartParams struct {
	ThreadID string `json:"threadId"`
}

// There are two ways to fork a thread: 1. By thread_id: load the thread from disk by
// thread_id and fork it into a new thread. 2. By path: load the thread from disk by path
// and fork it into a new thread.
//
// If using a non-empty path, the thread_id param will be ignored. Empty string path values
// are treated as absent.
//
// Prefer using thread_id whenever possible.
type ThreadForkParams struct {
	ApprovalPolicy                                                                         *ThreadForkParamsApprovalPolicy `json:"approvalPolicy"`
	// Override where approval requests are routed for review on this thread and subsequent                                
	// turns.                                                                                                              
	ApprovalsReviewer                                                                      *ApprovalsReviewer              `json:"approvalsReviewer"`
	BaseInstructions                                                                       *string                         `json:"baseInstructions"`
	Config                                                                                 map[string]interface{}          `json:"config"`
	Cwd                                                                                    *string                         `json:"cwd"`
	DeveloperInstructions                                                                  *string                         `json:"developerInstructions"`
	Ephemeral                                                                              *bool                           `json:"ephemeral,omitempty"`
	// Configuration overrides for the forked thread, if any.                                                              
	Model                                                                                  *string                         `json:"model"`
	ModelProvider                                                                          *string                         `json:"modelProvider"`
	Sandbox                                                                                *SandboxMode                    `json:"sandbox"`
	ServiceTier                                                                            *string                         `json:"serviceTier"`
	ThreadID                                                                               string                          `json:"threadId"`
	// Optional client-supplied analytics source classification for this forked thread.                                    
	ThreadSource                                                                           *ThreadSource                   `json:"threadSource"`
}

type FluffyGranularAskForApproval struct {
	Granular TentacledGranular `json:"granular"`
}

type TentacledGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

type ThreadForkResponse struct {
	ApprovalPolicy                                                                         *ThreadForkResponseAskForApproval `json:"approvalPolicy"`
	// Reviewer currently used for approval requests on this thread.                                                         
	ApprovalsReviewer                                                                      ApprovalsReviewer                 `json:"approvalsReviewer"`
	Cwd                                                                                    string                            `json:"cwd"`
	// Instruction source files currently loaded for this thread.                                                            
	InstructionSources                                                                     []string                          `json:"instructionSources,omitempty"`
	Model                                                                                  string                            `json:"model"`
	ModelProvider                                                                          string                            `json:"modelProvider"`
	ReasoningEffort                                                                        *ReasoningEffort                  `json:"reasoningEffort"`
	// Legacy sandbox policy retained for compatibility. Experimental clients should prefer                                  
	// `activePermissionProfile` for profile provenance.                                                                     
	Sandbox                                                                                ThreadForkResponseSandboxPolicy   `json:"sandbox"`
	ServiceTier                                                                            *string                           `json:"serviceTier"`
	Thread                                                                                 ThreadForkResponseThread          `json:"thread"`
}

type TentacledGranularAskForApproval struct {
	Granular StickyGranular `json:"granular"`
}

type StickyGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

// Legacy sandbox policy retained for compatibility. Experimental clients should prefer
// `activePermissionProfile` for profile provenance.
type ThreadForkResponseSandboxPolicy struct {
	Type                SandboxPolicyType   `json:"type"`
	NetworkAccess       *NetworkAccessUnion `json:"networkAccess"`
	ExcludeSlashTmp     *bool               `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool               `json:"excludeTmpdirEnvVar,omitempty"`
	WritableRoots       []string            `json:"writableRoots,omitempty"`
}

type ThreadForkResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                              
	AgentNickname                                                                            *string               `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                   
	AgentRole                                                                                *string               `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                 
	CLIVersion                                                                               string                `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                    
	CreatedAt                                                                                int64                 `json:"createdAt"`
	// Working directory captured for the thread.                                                                  
	Cwd                                                                                      string                `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                     
	Ephemeral                                                                                bool                  `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                    
	ForkedFromID                                                                             *string               `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                 
	GitInfo                                                                                  *PurpleGitInfo        `json:"gitInfo"`
	ID                                                                                       string                `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                                
	ModelProvider                                                                            string                `json:"modelProvider"`
	// Optional user-facing thread title.                                                                          
	Name                                                                                     *string               `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                      
	Path                                                                                     *string               `json:"path"`
	// Usually the first user message in the thread, if available.                                                 
	Preview                                                                                  string                `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                          
	SessionID                                                                                string                `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                     
	Source                                                                                   *MagentaSessionSource `json:"source"`
	// Current runtime status for the thread.                                                                      
	Status                                                                                   PurpleThreadStatus    `json:"status"`
	// Optional analytics source classification for this thread.                                                   
	ThreadSource                                                                             *ThreadSource         `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                      
	// (when `includeTurns` is true) responses. For all other responses and notifications                          
	// returning a Thread, the turns field will be an empty list.                                                  
	Turns                                                                                    []PurpleTurn          `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                               
	UpdatedAt                                                                                int64                 `json:"updatedAt"`
}

type PurpleGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type PurpleSessionSource struct {
	Custom   *string                `json:"custom,omitempty"`
	SubAgent *MagentaSubAgentSource `json:"subAgent"`
}

type PurpleSubAgentSource struct {
	ThreadSpawn *PurpleThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string            `json:"other,omitempty"`
}

type PurpleThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type PurpleThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type PurpleTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                    
	CompletedAt                                                             *int64             `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                   
	DurationMS                                                              *int64             `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                        
	Error                                                                   *FluffyTurnError   `json:"error"`
	ID                                                                      string             `json:"id"`
	// Thread items currently included in this turn payload.                                   
	Items                                                                   []FluffyThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                            
	ItemsView                                                               *TurnItemsView     `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                      
	StartedAt                                                               *int64             `json:"startedAt"`
	Status                                                                  TurnStatus         `json:"status"`
}

type FluffyTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo3 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type TentacledCodexErrorInfo struct {
	HTTPConnectionFailed           *TentacledHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *TentacledResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *TentacledResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *TentacledResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *TentacledActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type TentacledActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type TentacledHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type TentacledResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type TentacledResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type TentacledResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type FluffyThreadItem struct {
	Content                                                                                     []StickyContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                      
	ID                                                                                          string                                   `json:"id"`
	Type                                                                                        ThreadItemType                           `json:"type"`
	Fragments                                                                                   []StickyHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *StickyMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                            `json:"phase"`
	Text                                                                                        *string                                  `json:"text,omitempty"`
	Summary                                                                                     []string                                 `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                          
	AggregatedOutput                                                                            *string                                  `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                       
	Command                                                                                     *string                                  `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                            
	// returns a list of CommandAction objects because a single shell command may be composed of                                         
	// many commands piped together.                                                                                                     
	CommandActions                                                                              []StickyCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                  
	Cwd                                                                                         *string                                  `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                            
	//                                                                                                                                   
	// The duration of the MCP tool call in milliseconds.                                                                                
	//                                                                                                                                   
	// The duration of the dynamic tool call in milliseconds.                                                                            
	DurationMS                                                                                  *int64                                   `json:"durationMs"`
	// The command's exit code.                                                                                                          
	ExitCode                                                                                    *int64                                   `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                       
	ProcessID                                                                                   *string                                  `json:"processId"`
	Source                                                                                      *CommandExecutionSource                  `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                           
	Status                                                                                      *string                                  `json:"status,omitempty"`
	Changes                                                                                     []StickyFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                              `json:"arguments"`
	Error                                                                                       *StickyMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                  `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                  `json:"pluginId"`
	Result                                                                                      *StickyResult                            `json:"result"`
	Server                                                                                      *string                                  `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                         
	Tool                                                                                        *string                                  `json:"tool,omitempty"`
	ContentItems                                                                                []StickyDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                  `json:"namespace"`
	Success                                                                                     *bool                                    `json:"success"`
	// Last known status of the target agents, when available.                                                                           
	AgentsStates                                                                                map[string]StickyCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                           
	Model                                                                                       *string                                  `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                 
	Prompt                                                                                      *string                                  `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                
	ReasoningEffort                                                                             *ReasoningEffort                         `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                               
	// corresponds to the newly spawned agent.                                                                                           
	ReceiverThreadIDS                                                                           []string                                 `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                
	SenderThreadID                                                                              *string                                  `json:"senderThreadId,omitempty"`
	Action                                                                                      *StickyWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                  `json:"query,omitempty"`
	Path                                                                                        *string                                  `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                  `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                  `json:"savedPath"`
	Review                                                                                      *string                                  `json:"review,omitempty"`
}

type StickyWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type StickyCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type StickyFileUpdateChange struct {
	Diff string                `json:"diff"`
	Kind IndigoPatchChangeKind `json:"kind"`
	Path string                `json:"path"`
}

type IndigoPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type StickyCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type StickyUserInput struct {
	Text                                                                         *string             `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                    
	TextElements                                                                 []StickyTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType       `json:"type"`
	Detail                                                                       *ImageDetail        `json:"detail"`
	URL                                                                          *string             `json:"url,omitempty"`
	Path                                                                         *string             `json:"path,omitempty"`
	Name                                                                         *string             `json:"name,omitempty"`
}

type StickyTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                       
	ByteRange                                                                   StickyByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                
	Placeholder                                                                 *string         `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type StickyByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type StickyDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type StickyMCPToolCallError struct {
	Message string `json:"message"`
}

type StickyHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type StickyMemoryCitation struct {
	Entries   []StickyMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                    `json:"threadIds"`
}

type StickyMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type StickyMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadGoalClearParams struct {
	ThreadID string `json:"threadId"`
}

type ThreadGoalClearResponse struct {
	Cleared bool `json:"cleared"`
}

type ThreadGoalClearedNotification struct {
	ThreadID string `json:"threadId"`
}

type ThreadGoalGetParams struct {
	ThreadID string `json:"threadId"`
}

type ThreadGoalGetResponse struct {
	Goal *ThreadGoal `json:"goal"`
}

type ThreadGoal struct {
	CreatedAt       int64            `json:"createdAt"`
	Objective       string           `json:"objective"`
	Status          ThreadGoalStatus `json:"status"`
	ThreadID        string           `json:"threadId"`
	TimeUsedSeconds int64            `json:"timeUsedSeconds"`
	TokenBudget     *int64           `json:"tokenBudget"`
	TokensUsed      int64            `json:"tokensUsed"`
	UpdatedAt       int64            `json:"updatedAt"`
}

type ThreadGoalSetParams struct {
	Objective   *string           `json:"objective"`
	Status      *ThreadGoalStatus `json:"status"`
	ThreadID    string            `json:"threadId"`
	TokenBudget *int64            `json:"tokenBudget"`
}

type ThreadGoalSetResponse struct {
	Goal ThreadGoalSetResponseGoal `json:"goal"`
}

type ThreadGoalSetResponseGoal struct {
	CreatedAt       int64            `json:"createdAt"`
	Objective       string           `json:"objective"`
	Status          ThreadGoalStatus `json:"status"`
	ThreadID        string           `json:"threadId"`
	TimeUsedSeconds int64            `json:"timeUsedSeconds"`
	TokenBudget     *int64           `json:"tokenBudget"`
	TokensUsed      int64            `json:"tokensUsed"`
	UpdatedAt       int64            `json:"updatedAt"`
}

type ThreadGoalUpdatedNotification struct {
	Goal     ThreadGoalUpdatedNotificationGoal `json:"goal"`
	ThreadID string                            `json:"threadId"`
	TurnID   *string                           `json:"turnId"`
}

type ThreadGoalUpdatedNotificationGoal struct {
	CreatedAt       int64            `json:"createdAt"`
	Objective       string           `json:"objective"`
	Status          ThreadGoalStatus `json:"status"`
	ThreadID        string           `json:"threadId"`
	TimeUsedSeconds int64            `json:"timeUsedSeconds"`
	TokenBudget     *int64           `json:"tokenBudget"`
	TokensUsed      int64            `json:"tokensUsed"`
	UpdatedAt       int64            `json:"updatedAt"`
}

type ThreadInjectItemsParams struct {
	// Raw Responses API items to append to the thread's model-visible history.              
	Items                                                                      []interface{} `json:"items"`
	ThreadID                                                                   string        `json:"threadId"`
}

type ThreadListParams struct {
	// Optional archived filter; when set to true, only archived threads are returned. If false                           
	// or null, only non-archived threads are returned.                                                                   
	Archived                                                                                   *bool                      `json:"archived"`
	// Opaque pagination cursor returned by a previous call.                                                              
	Cursor                                                                                     *string                    `json:"cursor"`
	// Optional cwd filter or filters; when set, only threads whose session cwd exactly matches                           
	// one of these paths are returned.                                                                                   
	Cwd                                                                                        *ForcedChatgptWorkspaceIDS `json:"cwd"`
	// Optional page size; defaults to a reasonable server-side value.                                                    
	Limit                                                                                      *int64                     `json:"limit"`
	// Optional provider filter; when set, only sessions recorded under these providers are                               
	// returned. When present but empty, includes all providers.                                                          
	ModelProviders                                                                             []string                   `json:"modelProviders"`
	// Optional substring filter for the extracted thread title.                                                          
	SearchTerm                                                                                 *string                    `json:"searchTerm"`
	// Optional sort direction; defaults to descending (newest first).                                                    
	SortDirection                                                                              *SortDirection             `json:"sortDirection"`
	// Optional sort key; defaults to created_at.                                                                         
	SortKey                                                                                    *ThreadSortKey             `json:"sortKey"`
	// Optional source filter; when set, only sessions from these source kinds are returned.                              
	// When omitted or empty, defaults to interactive sources.                                                            
	SourceKinds                                                                                []ThreadSourceKind         `json:"sourceKinds"`
	// If true, return from the state DB without scanning JSONL rollouts to repair thread                                 
	// metadata. Omitted or false preserves scan-and-repair behavior.                                                     
	UseStateDBOnly                                                                             *bool                      `json:"useStateDbOnly,omitempty"`
}

type ThreadListResponse struct {
	// Opaque cursor to pass as `cursor` when reversing `sortDirection`. This is only populated                
	// when the page contains at least one thread. Use it with the opposite `sortDirection`; for               
	// timestamp sorts it anchors at the start of the page timestamp so same-second updates are                
	// not skipped.                                                                                            
	BackwardsCursor                                                                             *string        `json:"backwardsCursor"`
	Data                                                                                        []DatumElement `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. if None, there                  
	// are no more items to return.                                                                            
	NextCursor                                                                                  *string        `json:"nextCursor"`
}

type DatumElement struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                            
	AgentNickname                                                                            *string             `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                 
	AgentRole                                                                                *string             `json:"agentRole"`
	// Version of the CLI that created the thread.                                                               
	CLIVersion                                                                               string              `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                  
	CreatedAt                                                                                int64               `json:"createdAt"`
	// Working directory captured for the thread.                                                                
	Cwd                                                                                      string              `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                   
	Ephemeral                                                                                bool                `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                  
	ForkedFromID                                                                             *string             `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                               
	GitInfo                                                                                  *DatumGitInfo       `json:"gitInfo"`
	ID                                                                                       string              `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                              
	ModelProvider                                                                            string              `json:"modelProvider"`
	// Optional user-facing thread title.                                                                        
	Name                                                                                     *string             `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                    
	Path                                                                                     *string             `json:"path"`
	// Usually the first user message in the thread, if available.                                               
	Preview                                                                                  string              `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                        
	SessionID                                                                                string              `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                   
	Source                                                                                   *DatumSessionSource `json:"source"`
	// Current runtime status for the thread.                                                                    
	Status                                                                                   DatumThreadStatus   `json:"status"`
	// Optional analytics source classification for this thread.                                                 
	ThreadSource                                                                             *ThreadSource       `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                    
	// (when `includeTurns` is true) responses. For all other responses and notifications                        
	// returning a Thread, the turns field will be an empty list.                                                
	Turns                                                                                    []DatumTurn         `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                             
	UpdatedAt                                                                                int64               `json:"updatedAt"`
}

type DatumGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type FluffySessionSource struct {
	Custom   *string               `json:"custom,omitempty"`
	SubAgent *FriskySubAgentSource `json:"subAgent"`
}

type FluffySubAgentSource struct {
	ThreadSpawn *FluffyThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string            `json:"other,omitempty"`
}

type FluffyThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type DatumThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type DatumTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                       
	CompletedAt                                                             *int64                `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                      
	DurationMS                                                              *int64                `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                           
	Error                                                                   *TentacledTurnError   `json:"error"`
	ID                                                                      string                `json:"id"`
	// Thread items currently included in this turn payload.                                      
	Items                                                                   []TentacledThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                               
	ItemsView                                                               *TurnItemsView        `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                         
	StartedAt                                                               *int64                `json:"startedAt"`
	Status                                                                  TurnStatus            `json:"status"`
}

type TentacledTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo4 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type StickyCodexErrorInfo struct {
	HTTPConnectionFailed           *StickyHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *StickyResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *StickyResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *StickyResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *StickyActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type StickyActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type StickyHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type StickyResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type StickyResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type StickyResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type TentacledThreadItem struct {
	Content                                                                                     []IndigoContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                      
	ID                                                                                          string                                   `json:"id"`
	Type                                                                                        ThreadItemType                           `json:"type"`
	Fragments                                                                                   []IndigoHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *IndigoMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                            `json:"phase"`
	Text                                                                                        *string                                  `json:"text,omitempty"`
	Summary                                                                                     []string                                 `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                          
	AggregatedOutput                                                                            *string                                  `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                       
	Command                                                                                     *string                                  `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                            
	// returns a list of CommandAction objects because a single shell command may be composed of                                         
	// many commands piped together.                                                                                                     
	CommandActions                                                                              []IndigoCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                  
	Cwd                                                                                         *string                                  `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                            
	//                                                                                                                                   
	// The duration of the MCP tool call in milliseconds.                                                                                
	//                                                                                                                                   
	// The duration of the dynamic tool call in milliseconds.                                                                            
	DurationMS                                                                                  *int64                                   `json:"durationMs"`
	// The command's exit code.                                                                                                          
	ExitCode                                                                                    *int64                                   `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                       
	ProcessID                                                                                   *string                                  `json:"processId"`
	Source                                                                                      *CommandExecutionSource                  `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                           
	Status                                                                                      *string                                  `json:"status,omitempty"`
	Changes                                                                                     []IndigoFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                              `json:"arguments"`
	Error                                                                                       *IndigoMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                  `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                  `json:"pluginId"`
	Result                                                                                      *IndigoResult                            `json:"result"`
	Server                                                                                      *string                                  `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                         
	Tool                                                                                        *string                                  `json:"tool,omitempty"`
	ContentItems                                                                                []IndigoDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                  `json:"namespace"`
	Success                                                                                     *bool                                    `json:"success"`
	// Last known status of the target agents, when available.                                                                           
	AgentsStates                                                                                map[string]IndigoCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                           
	Model                                                                                       *string                                  `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                 
	Prompt                                                                                      *string                                  `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                
	ReasoningEffort                                                                             *ReasoningEffort                         `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                               
	// corresponds to the newly spawned agent.                                                                                           
	ReceiverThreadIDS                                                                           []string                                 `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                
	SenderThreadID                                                                              *string                                  `json:"senderThreadId,omitempty"`
	Action                                                                                      *IndigoWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                  `json:"query,omitempty"`
	Path                                                                                        *string                                  `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                  `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                  `json:"savedPath"`
	Review                                                                                      *string                                  `json:"review,omitempty"`
}

type IndigoWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type IndigoCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type IndigoFileUpdateChange struct {
	Diff string                  `json:"diff"`
	Kind IndecentPatchChangeKind `json:"kind"`
	Path string                  `json:"path"`
}

type IndecentPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type IndigoCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type IndigoUserInput struct {
	Text                                                                         *string             `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                    
	TextElements                                                                 []IndigoTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType       `json:"type"`
	Detail                                                                       *ImageDetail        `json:"detail"`
	URL                                                                          *string             `json:"url,omitempty"`
	Path                                                                         *string             `json:"path,omitempty"`
	Name                                                                         *string             `json:"name,omitempty"`
}

type IndigoTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                       
	ByteRange                                                                   IndigoByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                
	Placeholder                                                                 *string         `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type IndigoByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type IndigoDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type IndigoMCPToolCallError struct {
	Message string `json:"message"`
}

type IndigoHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type IndigoMemoryCitation struct {
	Entries   []IndigoMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                    `json:"threadIds"`
}

type IndigoMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type IndigoMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadLoadedListParams struct {
	// Opaque pagination cursor returned by a previous call.        
	Cursor                                                  *string `json:"cursor"`
	// Optional page size; defaults to no limit.                    
	Limit                                                   *int64  `json:"limit"`
}

type ThreadLoadedListResponse struct {
	// Thread ids for sessions currently loaded in memory.                                            
	Data                                                                                     []string `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item. if None, there         
	// are no more items to return.                                                                   
	NextCursor                                                                               *string  `json:"nextCursor"`
}

type ThreadMetadataUpdateParams struct {
	// Patch the stored Git metadata for this thread. Omit a field to leave it unchanged, set it                                   
	// to `null` to clear it, or provide a string to replace the stored value.                                                     
	GitInfo                                                                                     *ThreadMetadataGitInfoUpdateParams `json:"gitInfo"`
	ThreadID                                                                                    string                             `json:"threadId"`
}

type ThreadMetadataGitInfoUpdateParams struct {
	// Omit to leave the stored branch unchanged, set to `null` to clear it, or provide a            
	// non-empty string to replace it.                                                               
	Branch                                                                                   *string `json:"branch"`
	// Omit to leave the stored origin URL unchanged, set to `null` to clear it, or provide a        
	// non-empty string to replace it.                                                               
	OriginURL                                                                                *string `json:"originUrl"`
	// Omit to leave the stored commit unchanged, set to `null` to clear it, or provide a            
	// non-empty string to replace it.                                                               
	SHA                                                                                      *string `json:"sha"`
}

type ThreadMetadataUpdateResponse struct {
	Thread ThreadMetadataUpdateResponseThread `json:"thread"`
}

type ThreadMetadataUpdateResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                             
	AgentNickname                                                                            *string              `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                  
	AgentRole                                                                                *string              `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                
	CLIVersion                                                                               string               `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                   
	CreatedAt                                                                                int64                `json:"createdAt"`
	// Working directory captured for the thread.                                                                 
	Cwd                                                                                      string               `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                    
	Ephemeral                                                                                bool                 `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                   
	ForkedFromID                                                                             *string              `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                
	GitInfo                                                                                  *FluffyGitInfo       `json:"gitInfo"`
	ID                                                                                       string               `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                               
	ModelProvider                                                                            string               `json:"modelProvider"`
	// Optional user-facing thread title.                                                                         
	Name                                                                                     *string              `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                     
	Path                                                                                     *string              `json:"path"`
	// Usually the first user message in the thread, if available.                                                
	Preview                                                                                  string               `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                         
	SessionID                                                                                string               `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                    
	Source                                                                                   *FriskySessionSource `json:"source"`
	// Current runtime status for the thread.                                                                     
	Status                                                                                   FluffyThreadStatus   `json:"status"`
	// Optional analytics source classification for this thread.                                                  
	ThreadSource                                                                             *ThreadSource        `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                     
	// (when `includeTurns` is true) responses. For all other responses and notifications                         
	// returning a Thread, the turns field will be an empty list.                                                 
	Turns                                                                                    []FluffyTurn         `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                              
	UpdatedAt                                                                                int64                `json:"updatedAt"`
}

type FluffyGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type TentacledSessionSource struct {
	Custom   *string                    `json:"custom,omitempty"`
	SubAgent *MischievousSubAgentSource `json:"subAgent"`
}

type TentacledSubAgentSource struct {
	ThreadSpawn *TentacledThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string               `json:"other,omitempty"`
}

type TentacledThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type FluffyThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type FluffyTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                    
	CompletedAt                                                             *int64             `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                   
	DurationMS                                                              *int64             `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                        
	Error                                                                   *StickyTurnError   `json:"error"`
	ID                                                                      string             `json:"id"`
	// Thread items currently included in this turn payload.                                   
	Items                                                                   []StickyThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                            
	ItemsView                                                               *TurnItemsView     `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                      
	StartedAt                                                               *int64             `json:"startedAt"`
	Status                                                                  TurnStatus         `json:"status"`
}

type StickyTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo5 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type IndigoCodexErrorInfo struct {
	HTTPConnectionFailed           *IndigoHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *IndigoResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *IndigoResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *IndigoResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *IndigoActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type IndigoActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type IndigoHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type IndigoResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type IndigoResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type IndigoResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type StickyThreadItem struct {
	Content                                                                                     []IndecentContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                        
	ID                                                                                          string                                     `json:"id"`
	Type                                                                                        ThreadItemType                             `json:"type"`
	Fragments                                                                                   []IndecentHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *IndecentMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                              `json:"phase"`
	Text                                                                                        *string                                    `json:"text,omitempty"`
	Summary                                                                                     []string                                   `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                            
	AggregatedOutput                                                                            *string                                    `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                         
	Command                                                                                     *string                                    `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                              
	// returns a list of CommandAction objects because a single shell command may be composed of                                           
	// many commands piped together.                                                                                                       
	CommandActions                                                                              []IndecentCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                    
	Cwd                                                                                         *string                                    `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                              
	//                                                                                                                                     
	// The duration of the MCP tool call in milliseconds.                                                                                  
	//                                                                                                                                     
	// The duration of the dynamic tool call in milliseconds.                                                                              
	DurationMS                                                                                  *int64                                     `json:"durationMs"`
	// The command's exit code.                                                                                                            
	ExitCode                                                                                    *int64                                     `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                         
	ProcessID                                                                                   *string                                    `json:"processId"`
	Source                                                                                      *CommandExecutionSource                    `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                             
	Status                                                                                      *string                                    `json:"status,omitempty"`
	Changes                                                                                     []IndecentFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                                `json:"arguments"`
	Error                                                                                       *IndecentMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                    `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                    `json:"pluginId"`
	Result                                                                                      *IndecentResult                            `json:"result"`
	Server                                                                                      *string                                    `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                           
	Tool                                                                                        *string                                    `json:"tool,omitempty"`
	ContentItems                                                                                []IndecentDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                    `json:"namespace"`
	Success                                                                                     *bool                                      `json:"success"`
	// Last known status of the target agents, when available.                                                                             
	AgentsStates                                                                                map[string]IndecentCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                             
	Model                                                                                       *string                                    `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                   
	Prompt                                                                                      *string                                    `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                  
	ReasoningEffort                                                                             *ReasoningEffort                           `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                 
	// corresponds to the newly spawned agent.                                                                                             
	ReceiverThreadIDS                                                                           []string                                   `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                  
	SenderThreadID                                                                              *string                                    `json:"senderThreadId,omitempty"`
	Action                                                                                      *IndecentWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                    `json:"query,omitempty"`
	Path                                                                                        *string                                    `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                    `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                    `json:"savedPath"`
	Review                                                                                      *string                                    `json:"review,omitempty"`
}

type IndecentWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type IndecentCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type IndecentFileUpdateChange struct {
	Diff string                   `json:"diff"`
	Kind HilariousPatchChangeKind `json:"kind"`
	Path string                   `json:"path"`
}

type HilariousPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type IndecentCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type IndecentUserInput struct {
	Text                                                                         *string               `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                      
	TextElements                                                                 []IndecentTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType         `json:"type"`
	Detail                                                                       *ImageDetail          `json:"detail"`
	URL                                                                          *string               `json:"url,omitempty"`
	Path                                                                         *string               `json:"path,omitempty"`
	Name                                                                         *string               `json:"name,omitempty"`
}

type IndecentTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                         
	ByteRange                                                                   IndecentByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                  
	Placeholder                                                                 *string           `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type IndecentByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type IndecentDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type IndecentMCPToolCallError struct {
	Message string `json:"message"`
}

type IndecentHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type IndecentMemoryCitation struct {
	Entries   []IndecentMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                      `json:"threadIds"`
}

type IndecentMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type IndecentMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadNameUpdatedNotification struct {
	ThreadID   string  `json:"threadId"`
	ThreadName *string `json:"threadName"`
}

type ThreadReadParams struct {
	// When true, include turns and their items from rollout history.       
	IncludeTurns                                                     *bool  `json:"includeTurns,omitempty"`
	ThreadID                                                         string `json:"threadId"`
}

type ThreadReadResponse struct {
	Thread ThreadReadResponseThread `json:"thread"`
}

type ThreadReadResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                                  
	AgentNickname                                                                            *string                   `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                       
	AgentRole                                                                                *string                   `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                     
	CLIVersion                                                                               string                    `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                        
	CreatedAt                                                                                int64                     `json:"createdAt"`
	// Working directory captured for the thread.                                                                      
	Cwd                                                                                      string                    `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                         
	Ephemeral                                                                                bool                      `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                        
	ForkedFromID                                                                             *string                   `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                     
	GitInfo                                                                                  *TentacledGitInfo         `json:"gitInfo"`
	ID                                                                                       string                    `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                                    
	ModelProvider                                                                            string                    `json:"modelProvider"`
	// Optional user-facing thread title.                                                                              
	Name                                                                                     *string                   `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                          
	Path                                                                                     *string                   `json:"path"`
	// Usually the first user message in the thread, if available.                                                     
	Preview                                                                                  string                    `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                              
	SessionID                                                                                string                    `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                         
	Source                                                                                   *MischievousSessionSource `json:"source"`
	// Current runtime status for the thread.                                                                          
	Status                                                                                   TentacledThreadStatus     `json:"status"`
	// Optional analytics source classification for this thread.                                                       
	ThreadSource                                                                             *ThreadSource             `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                          
	// (when `includeTurns` is true) responses. For all other responses and notifications                              
	// returning a Thread, the turns field will be an empty list.                                                      
	Turns                                                                                    []TentacledTurn           `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                                   
	UpdatedAt                                                                                int64                     `json:"updatedAt"`
}

type TentacledGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type StickySessionSource struct {
	Custom   *string                      `json:"custom,omitempty"`
	SubAgent *BraggadociousSubAgentSource `json:"subAgent"`
}

type StickySubAgentSource struct {
	ThreadSpawn *StickyThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string            `json:"other,omitempty"`
}

type StickyThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type TentacledThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type TentacledTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                    
	CompletedAt                                                             *int64             `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                   
	DurationMS                                                              *int64             `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                        
	Error                                                                   *IndigoTurnError   `json:"error"`
	ID                                                                      string             `json:"id"`
	// Thread items currently included in this turn payload.                                   
	Items                                                                   []IndigoThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                            
	ItemsView                                                               *TurnItemsView     `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                      
	StartedAt                                                               *int64             `json:"startedAt"`
	Status                                                                  TurnStatus         `json:"status"`
}

type IndigoTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo6 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type IndecentCodexErrorInfo struct {
	HTTPConnectionFailed           *IndecentHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *IndecentResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *IndecentResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *IndecentResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *IndecentActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type IndecentActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type IndecentHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type IndecentResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type IndecentResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type IndecentResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type IndigoThreadItem struct {
	Content                                                                                     []HilariousContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                         
	ID                                                                                          string                                      `json:"id"`
	Type                                                                                        ThreadItemType                              `json:"type"`
	Fragments                                                                                   []HilariousHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *HilariousMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                               `json:"phase"`
	Text                                                                                        *string                                     `json:"text,omitempty"`
	Summary                                                                                     []string                                    `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                             
	AggregatedOutput                                                                            *string                                     `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                          
	Command                                                                                     *string                                     `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                               
	// returns a list of CommandAction objects because a single shell command may be composed of                                            
	// many commands piped together.                                                                                                        
	CommandActions                                                                              []HilariousCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                     
	Cwd                                                                                         *string                                     `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                               
	//                                                                                                                                      
	// The duration of the MCP tool call in milliseconds.                                                                                   
	//                                                                                                                                      
	// The duration of the dynamic tool call in milliseconds.                                                                               
	DurationMS                                                                                  *int64                                      `json:"durationMs"`
	// The command's exit code.                                                                                                             
	ExitCode                                                                                    *int64                                      `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                          
	ProcessID                                                                                   *string                                     `json:"processId"`
	Source                                                                                      *CommandExecutionSource                     `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                              
	Status                                                                                      *string                                     `json:"status,omitempty"`
	Changes                                                                                     []HilariousFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                                 `json:"arguments"`
	Error                                                                                       *HilariousMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                     `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                     `json:"pluginId"`
	Result                                                                                      *HilariousResult                            `json:"result"`
	Server                                                                                      *string                                     `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                            
	Tool                                                                                        *string                                     `json:"tool,omitempty"`
	ContentItems                                                                                []HilariousDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                     `json:"namespace"`
	Success                                                                                     *bool                                       `json:"success"`
	// Last known status of the target agents, when available.                                                                              
	AgentsStates                                                                                map[string]HilariousCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                              
	Model                                                                                       *string                                     `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                    
	Prompt                                                                                      *string                                     `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                   
	ReasoningEffort                                                                             *ReasoningEffort                            `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                  
	// corresponds to the newly spawned agent.                                                                                              
	ReceiverThreadIDS                                                                           []string                                    `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                   
	SenderThreadID                                                                              *string                                     `json:"senderThreadId,omitempty"`
	Action                                                                                      *HilariousWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                     `json:"query,omitempty"`
	Path                                                                                        *string                                     `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                     `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                     `json:"savedPath"`
	Review                                                                                      *string                                     `json:"review,omitempty"`
}

type HilariousWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type HilariousCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type HilariousFileUpdateChange struct {
	Diff string                   `json:"diff"`
	Kind AmbitiousPatchChangeKind `json:"kind"`
	Path string                   `json:"path"`
}

type AmbitiousPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type HilariousCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type HilariousUserInput struct {
	Text                                                                         *string                `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                       
	TextElements                                                                 []HilariousTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType          `json:"type"`
	Detail                                                                       *ImageDetail           `json:"detail"`
	URL                                                                          *string                `json:"url,omitempty"`
	Path                                                                         *string                `json:"path,omitempty"`
	Name                                                                         *string                `json:"name,omitempty"`
}

type HilariousTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                          
	ByteRange                                                                   HilariousByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                   
	Placeholder                                                                 *string            `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type HilariousByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type HilariousDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type HilariousMCPToolCallError struct {
	Message string `json:"message"`
}

type HilariousHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type HilariousMemoryCitation struct {
	Entries   []HilariousMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                       `json:"threadIds"`
}

type HilariousMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type HilariousMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

// EXPERIMENTAL - emitted when thread realtime transport closes.
type ThreadRealtimeClosedNotification struct {
	Reason   *string `json:"reason"`
	ThreadID string  `json:"threadId"`
}

// EXPERIMENTAL - emitted when thread realtime encounters an error.
type ThreadRealtimeErrorNotification struct {
	Message  string `json:"message"`
	ThreadID string `json:"threadId"`
}

// EXPERIMENTAL - raw non-audio thread realtime item emitted by the backend.
type ThreadRealtimeItemAddedNotification struct {
	Item     interface{} `json:"item"`
	ThreadID string      `json:"threadId"`
}

// EXPERIMENTAL - streamed output audio emitted by thread realtime.
type ThreadRealtimeOutputAudioDeltaNotification struct {
	Audio    ThreadRealtimeAudioChunk `json:"audio"`
	ThreadID string                   `json:"threadId"`
}

// EXPERIMENTAL - thread realtime audio chunk.
type ThreadRealtimeAudioChunk struct {
	Data              string  `json:"data"`
	ItemID            *string `json:"itemId"`
	NumChannels       int64   `json:"numChannels"`
	SampleRate        int64   `json:"sampleRate"`
	SamplesPerChannel *int64  `json:"samplesPerChannel"`
}

// EXPERIMENTAL - emitted with the remote SDP for a WebRTC realtime session.
type ThreadRealtimeSDPNotification struct {
	SDP      string `json:"sdp"`
	ThreadID string `json:"threadId"`
}

// EXPERIMENTAL - emitted when thread realtime startup is accepted.
type ThreadRealtimeStartedNotification struct {
	RealtimeSessionID *string                     `json:"realtimeSessionId"`
	ThreadID          string                      `json:"threadId"`
	Version           RealtimeConversationVersion `json:"version"`
}

// EXPERIMENTAL - flat transcript delta emitted whenever realtime transcript text changes.
type ThreadRealtimeTranscriptDeltaNotification struct {
	// Live transcript delta from the realtime event.       
	Delta                                            string `json:"delta"`
	Role                                             string `json:"role"`
	ThreadID                                         string `json:"threadId"`
}

// EXPERIMENTAL - final transcript text emitted when realtime completes a transcript part.
type ThreadRealtimeTranscriptDoneNotification struct {
	Role                                           string `json:"role"`
	// Final complete text for the transcript part.       
	Text                                           string `json:"text"`
	ThreadID                                       string `json:"threadId"`
}

// There are three ways to resume a thread: 1. By thread_id: load the thread from disk by
// thread_id and resume it. 2. By history: instantiate the thread from memory and resume it.
// 3. By path: load the thread from disk by path and resume it.
//
// For non-running threads, the precedence is: history > non-empty path > thread_id. If
// using history or a non-empty path for a non-running thread, the thread_id param will be
// ignored.
//
// If thread_id identifies a running thread, app-server rejoins that thread and treats a
// non-empty path as a consistency check against the active rollout path. Empty string path
// values are treated as absent.
//
// Prefer using thread_id whenever possible.
type ThreadResumeParams struct {
	ApprovalPolicy                                                                         *ThreadResumeParamsApprovalPolicy `json:"approvalPolicy"`
	// Override where approval requests are routed for review on this thread and subsequent                                  
	// turns.                                                                                                                
	ApprovalsReviewer                                                                      *ApprovalsReviewer                `json:"approvalsReviewer"`
	BaseInstructions                                                                       *string                           `json:"baseInstructions"`
	Config                                                                                 map[string]interface{}            `json:"config"`
	Cwd                                                                                    *string                           `json:"cwd"`
	DeveloperInstructions                                                                  *string                           `json:"developerInstructions"`
	// Configuration overrides for the resumed thread, if any.                                                               
	Model                                                                                  *string                           `json:"model"`
	ModelProvider                                                                          *string                           `json:"modelProvider"`
	Personality                                                                            *Personality                      `json:"personality"`
	Sandbox                                                                                *SandboxMode                      `json:"sandbox"`
	ServiceTier                                                                            *string                           `json:"serviceTier"`
	ThreadID                                                                               string                            `json:"threadId"`
}

type StickyGranularAskForApproval struct {
	Granular IndigoGranular `json:"granular"`
}

type IndigoGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

type ThreadResumeResponse struct {
	ApprovalPolicy                                                                         *ThreadResumeResponseAskForApproval `json:"approvalPolicy"`
	// Reviewer currently used for approval requests on this thread.                                                           
	ApprovalsReviewer                                                                      ApprovalsReviewer                   `json:"approvalsReviewer"`
	Cwd                                                                                    string                              `json:"cwd"`
	// Instruction source files currently loaded for this thread.                                                              
	InstructionSources                                                                     []string                            `json:"instructionSources,omitempty"`
	Model                                                                                  string                              `json:"model"`
	ModelProvider                                                                          string                              `json:"modelProvider"`
	ReasoningEffort                                                                        *ReasoningEffort                    `json:"reasoningEffort"`
	// Legacy sandbox policy retained for compatibility. Experimental clients should prefer                                    
	// `activePermissionProfile` for profile provenance.                                                                       
	Sandbox                                                                                ThreadResumeResponseSandboxPolicy   `json:"sandbox"`
	ServiceTier                                                                            *string                             `json:"serviceTier"`
	Thread                                                                                 ThreadResumeResponseThread          `json:"thread"`
}

type IndigoGranularAskForApproval struct {
	Granular IndecentGranular `json:"granular"`
}

type IndecentGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

// Legacy sandbox policy retained for compatibility. Experimental clients should prefer
// `activePermissionProfile` for profile provenance.
type ThreadResumeResponseSandboxPolicy struct {
	Type                SandboxPolicyType   `json:"type"`
	NetworkAccess       *NetworkAccessUnion `json:"networkAccess"`
	ExcludeSlashTmp     *bool               `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool               `json:"excludeTmpdirEnvVar,omitempty"`
	WritableRoots       []string            `json:"writableRoots,omitempty"`
}

type ThreadResumeResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                                    
	AgentNickname                                                                            *string                     `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                         
	AgentRole                                                                                *string                     `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                       
	CLIVersion                                                                               string                      `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                          
	CreatedAt                                                                                int64                       `json:"createdAt"`
	// Working directory captured for the thread.                                                                        
	Cwd                                                                                      string                      `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                           
	Ephemeral                                                                                bool                        `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                          
	ForkedFromID                                                                             *string                     `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                       
	GitInfo                                                                                  *StickyGitInfo              `json:"gitInfo"`
	ID                                                                                       string                      `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                                      
	ModelProvider                                                                            string                      `json:"modelProvider"`
	// Optional user-facing thread title.                                                                                
	Name                                                                                     *string                     `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                            
	Path                                                                                     *string                     `json:"path"`
	// Usually the first user message in the thread, if available.                                                       
	Preview                                                                                  string                      `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                                
	SessionID                                                                                string                      `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                           
	Source                                                                                   *BraggadociousSessionSource `json:"source"`
	// Current runtime status for the thread.                                                                            
	Status                                                                                   StickyThreadStatus          `json:"status"`
	// Optional analytics source classification for this thread.                                                         
	ThreadSource                                                                             *ThreadSource               `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                            
	// (when `includeTurns` is true) responses. For all other responses and notifications                                
	// returning a Thread, the turns field will be an empty list.                                                        
	Turns                                                                                    []StickyTurn                `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                                     
	UpdatedAt                                                                                int64                       `json:"updatedAt"`
}

type StickyGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type IndigoSessionSource struct {
	Custom   *string          `json:"custom,omitempty"`
	SubAgent *SubAgentSource1 `json:"subAgent"`
}

type IndigoSubAgentSource struct {
	ThreadSpawn *IndigoThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string            `json:"other,omitempty"`
}

type IndigoThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type StickyThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type StickyTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                      
	CompletedAt                                                             *int64               `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                     
	DurationMS                                                              *int64               `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                          
	Error                                                                   *IndecentTurnError   `json:"error"`
	ID                                                                      string               `json:"id"`
	// Thread items currently included in this turn payload.                                     
	Items                                                                   []IndecentThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                              
	ItemsView                                                               *TurnItemsView       `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                        
	StartedAt                                                               *int64               `json:"startedAt"`
	Status                                                                  TurnStatus           `json:"status"`
}

type IndecentTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo7 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type HilariousCodexErrorInfo struct {
	HTTPConnectionFailed           *HilariousHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *HilariousResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *HilariousResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *HilariousResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *HilariousActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type HilariousActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type HilariousHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type HilariousResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type HilariousResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type HilariousResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type IndecentThreadItem struct {
	Content                                                                                     []AmbitiousContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                         
	ID                                                                                          string                                      `json:"id"`
	Type                                                                                        ThreadItemType                              `json:"type"`
	Fragments                                                                                   []AmbitiousHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *AmbitiousMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                               `json:"phase"`
	Text                                                                                        *string                                     `json:"text,omitempty"`
	Summary                                                                                     []string                                    `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                             
	AggregatedOutput                                                                            *string                                     `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                          
	Command                                                                                     *string                                     `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                               
	// returns a list of CommandAction objects because a single shell command may be composed of                                            
	// many commands piped together.                                                                                                        
	CommandActions                                                                              []AmbitiousCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                     
	Cwd                                                                                         *string                                     `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                               
	//                                                                                                                                      
	// The duration of the MCP tool call in milliseconds.                                                                                   
	//                                                                                                                                      
	// The duration of the dynamic tool call in milliseconds.                                                                               
	DurationMS                                                                                  *int64                                      `json:"durationMs"`
	// The command's exit code.                                                                                                             
	ExitCode                                                                                    *int64                                      `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                          
	ProcessID                                                                                   *string                                     `json:"processId"`
	Source                                                                                      *CommandExecutionSource                     `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                              
	Status                                                                                      *string                                     `json:"status,omitempty"`
	Changes                                                                                     []AmbitiousFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                                 `json:"arguments"`
	Error                                                                                       *AmbitiousMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                     `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                     `json:"pluginId"`
	Result                                                                                      *AmbitiousResult                            `json:"result"`
	Server                                                                                      *string                                     `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                            
	Tool                                                                                        *string                                     `json:"tool,omitempty"`
	ContentItems                                                                                []AmbitiousDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                     `json:"namespace"`
	Success                                                                                     *bool                                       `json:"success"`
	// Last known status of the target agents, when available.                                                                              
	AgentsStates                                                                                map[string]AmbitiousCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                              
	Model                                                                                       *string                                     `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                    
	Prompt                                                                                      *string                                     `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                   
	ReasoningEffort                                                                             *ReasoningEffort                            `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                  
	// corresponds to the newly spawned agent.                                                                                              
	ReceiverThreadIDS                                                                           []string                                    `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                   
	SenderThreadID                                                                              *string                                     `json:"senderThreadId,omitempty"`
	Action                                                                                      *AmbitiousWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                     `json:"query,omitempty"`
	Path                                                                                        *string                                     `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                     `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                     `json:"savedPath"`
	Review                                                                                      *string                                     `json:"review,omitempty"`
}

type AmbitiousWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type AmbitiousCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type AmbitiousFileUpdateChange struct {
	Diff string                 `json:"diff"`
	Kind CunningPatchChangeKind `json:"kind"`
	Path string                 `json:"path"`
}

type CunningPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type AmbitiousCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type AmbitiousUserInput struct {
	Text                                                                         *string                `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                       
	TextElements                                                                 []AmbitiousTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType          `json:"type"`
	Detail                                                                       *ImageDetail           `json:"detail"`
	URL                                                                          *string                `json:"url,omitempty"`
	Path                                                                         *string                `json:"path,omitempty"`
	Name                                                                         *string                `json:"name,omitempty"`
}

type AmbitiousTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                          
	ByteRange                                                                   AmbitiousByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                   
	Placeholder                                                                 *string            `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type AmbitiousByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type AmbitiousDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type AmbitiousMCPToolCallError struct {
	Message string `json:"message"`
}

type AmbitiousHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type AmbitiousMemoryCitation struct {
	Entries   []AmbitiousMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                       `json:"threadIds"`
}

type AmbitiousMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type AmbitiousMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadRollbackParams struct {
	// The number of turns to drop from the end of the thread. Must be >= 1.                          
	//                                                                                                
	// This only modifies the thread's history and does not revert local file changes that have       
	// been made by the agent. Clients are responsible for reverting these changes.                   
	NumTurns                                                                                   int64  `json:"numTurns"`
	ThreadID                                                                                   string `json:"threadId"`
}

type ThreadRollbackResponse struct {
	// The updated thread after applying the rollback, with `turns` populated.                                          
	//                                                                                                                  
	// The ThreadItems stored in each Turn are lossy since we explicitly do not persist all                             
	// agent interactions, such as command executions. This is the same behavior as                                     
	// `thread/resume`.                                                                                                 
	Thread                                                                                 ThreadRollbackResponseThread `json:"thread"`
}

// The updated thread after applying the rollback, with `turns` populated.
//
// The ThreadItems stored in each Turn are lossy since we explicitly do not persist all
// agent interactions, such as command executions. This is the same behavior as
// `thread/resume`.
type ThreadRollbackResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                           
	AgentNickname                                                                            *string            `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                
	AgentRole                                                                                *string            `json:"agentRole"`
	// Version of the CLI that created the thread.                                                              
	CLIVersion                                                                               string             `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                 
	CreatedAt                                                                                int64              `json:"createdAt"`
	// Working directory captured for the thread.                                                               
	Cwd                                                                                      string             `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                  
	Ephemeral                                                                                bool               `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                 
	ForkedFromID                                                                             *string            `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                              
	GitInfo                                                                                  *IndigoGitInfo     `json:"gitInfo"`
	ID                                                                                       string             `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                             
	ModelProvider                                                                            string             `json:"modelProvider"`
	// Optional user-facing thread title.                                                                       
	Name                                                                                     *string            `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                   
	Path                                                                                     *string            `json:"path"`
	// Usually the first user message in the thread, if available.                                              
	Preview                                                                                  string             `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                       
	SessionID                                                                                string             `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                  
	Source                                                                                   *SessionSource1    `json:"source"`
	// Current runtime status for the thread.                                                                   
	Status                                                                                   IndigoThreadStatus `json:"status"`
	// Optional analytics source classification for this thread.                                                
	ThreadSource                                                                             *ThreadSource      `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                   
	// (when `includeTurns` is true) responses. For all other responses and notifications                       
	// returning a Thread, the turns field will be an empty list.                                               
	Turns                                                                                    []IndigoTurn       `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                            
	UpdatedAt                                                                                int64              `json:"updatedAt"`
}

type IndigoGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type IndecentSessionSource struct {
	Custom   *string          `json:"custom,omitempty"`
	SubAgent *SubAgentSource2 `json:"subAgent"`
}

type IndecentSubAgentSource struct {
	ThreadSpawn *IndecentThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string              `json:"other,omitempty"`
}

type IndecentThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type IndigoThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type IndigoTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                       
	CompletedAt                                                             *int64                `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                      
	DurationMS                                                              *int64                `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                           
	Error                                                                   *HilariousTurnError   `json:"error"`
	ID                                                                      string                `json:"id"`
	// Thread items currently included in this turn payload.                                      
	Items                                                                   []HilariousThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                               
	ItemsView                                                               *TurnItemsView        `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                         
	StartedAt                                                               *int64                `json:"startedAt"`
	Status                                                                  TurnStatus            `json:"status"`
}

type HilariousTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo8 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type AmbitiousCodexErrorInfo struct {
	HTTPConnectionFailed           *AmbitiousHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *AmbitiousResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *AmbitiousResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *AmbitiousResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *AmbitiousActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type AmbitiousActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type AmbitiousHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type AmbitiousResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type AmbitiousResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type AmbitiousResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type HilariousThreadItem struct {
	Content                                                                                     []CunningContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                       
	ID                                                                                          string                                    `json:"id"`
	Type                                                                                        ThreadItemType                            `json:"type"`
	Fragments                                                                                   []CunningHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *CunningMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                             `json:"phase"`
	Text                                                                                        *string                                   `json:"text,omitempty"`
	Summary                                                                                     []string                                  `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                           
	AggregatedOutput                                                                            *string                                   `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                        
	Command                                                                                     *string                                   `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                             
	// returns a list of CommandAction objects because a single shell command may be composed of                                          
	// many commands piped together.                                                                                                      
	CommandActions                                                                              []CunningCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                   
	Cwd                                                                                         *string                                   `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                             
	//                                                                                                                                    
	// The duration of the MCP tool call in milliseconds.                                                                                 
	//                                                                                                                                    
	// The duration of the dynamic tool call in milliseconds.                                                                             
	DurationMS                                                                                  *int64                                    `json:"durationMs"`
	// The command's exit code.                                                                                                           
	ExitCode                                                                                    *int64                                    `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                        
	ProcessID                                                                                   *string                                   `json:"processId"`
	Source                                                                                      *CommandExecutionSource                   `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                            
	Status                                                                                      *string                                   `json:"status,omitempty"`
	Changes                                                                                     []CunningFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                               `json:"arguments"`
	Error                                                                                       *CunningMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                   `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                   `json:"pluginId"`
	Result                                                                                      *CunningResult                            `json:"result"`
	Server                                                                                      *string                                   `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                          
	Tool                                                                                        *string                                   `json:"tool,omitempty"`
	ContentItems                                                                                []CunningDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                   `json:"namespace"`
	Success                                                                                     *bool                                     `json:"success"`
	// Last known status of the target agents, when available.                                                                            
	AgentsStates                                                                                map[string]CunningCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                            
	Model                                                                                       *string                                   `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                  
	Prompt                                                                                      *string                                   `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                 
	ReasoningEffort                                                                             *ReasoningEffort                          `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                
	// corresponds to the newly spawned agent.                                                                                            
	ReceiverThreadIDS                                                                           []string                                  `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                 
	SenderThreadID                                                                              *string                                   `json:"senderThreadId,omitempty"`
	Action                                                                                      *CunningWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                   `json:"query,omitempty"`
	Path                                                                                        *string                                   `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                   `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                   `json:"savedPath"`
	Review                                                                                      *string                                   `json:"review,omitempty"`
}

type CunningWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type CunningCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type CunningFileUpdateChange struct {
	Diff string                 `json:"diff"`
	Kind MagentaPatchChangeKind `json:"kind"`
	Path string                 `json:"path"`
}

type MagentaPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type CunningCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type CunningUserInput struct {
	Text                                                                         *string              `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                     
	TextElements                                                                 []CunningTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType        `json:"type"`
	Detail                                                                       *ImageDetail         `json:"detail"`
	URL                                                                          *string              `json:"url,omitempty"`
	Path                                                                         *string              `json:"path,omitempty"`
	Name                                                                         *string              `json:"name,omitempty"`
}

type CunningTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                        
	ByteRange                                                                   CunningByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                 
	Placeholder                                                                 *string          `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type CunningByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type CunningDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type CunningMCPToolCallError struct {
	Message string `json:"message"`
}

type CunningHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type CunningMemoryCitation struct {
	Entries   []CunningMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                     `json:"threadIds"`
}

type CunningMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type CunningMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadSetNameParams struct {
	Name     string `json:"name"`
	ThreadID string `json:"threadId"`
}

type ThreadSettingsUpdatedNotification struct {
	ThreadID       string         `json:"threadId"`
	ThreadSettings ThreadSettings `json:"threadSettings"`
}

type ThreadSettings struct {
	ActivePermissionProfile *ActivePermissionProfile      `json:"activePermissionProfile"`
	ApprovalPolicy          *ThreadSettingsAskForApproval `json:"approvalPolicy"`
	ApprovalsReviewer       ApprovalsReviewer             `json:"approvalsReviewer"`
	CollaborationMode       CollaborationMode             `json:"collaborationMode"`
	Cwd                     string                        `json:"cwd"`
	Effort                  *ReasoningEffort              `json:"effort"`
	Model                   string                        `json:"model"`
	ModelProvider           string                        `json:"modelProvider"`
	Personality             *Personality                  `json:"personality"`
	SandboxPolicy           ThreadSettingsSandboxPolicy   `json:"sandboxPolicy"`
	ServiceTier             *string                       `json:"serviceTier"`
	Summary                 *ReasoningSummary             `json:"summary"`
}

type ActivePermissionProfile struct {
	// Parent profile identifier from the selected permissions profile's `extends` setting, when        
	// present.                                                                                         
	Extends                                                                                     *string `json:"extends"`
	// Identifier from `default_permissions` or the implicit built-in default, such as                  
	// `:workspace` or a user-defined `[permissions.<id>]` profile.                                     
	ID                                                                                          string  `json:"id"`
}

type IndecentGranularAskForApproval struct {
	Granular HilariousGranular `json:"granular"`
}

type HilariousGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

// Collaboration mode for a Codex session.
type CollaborationMode struct {
	Mode     ModeKind `json:"mode"`
	Settings Settings `json:"settings"`
}

// Settings for a collaboration mode.
type Settings struct {
	DeveloperInstructions *string          `json:"developer_instructions"`
	Model                 string           `json:"model"`
	ReasoningEffort       *ReasoningEffort `json:"reasoning_effort"`
}

type ThreadSettingsSandboxPolicy struct {
	Type                SandboxPolicyType   `json:"type"`
	NetworkAccess       *NetworkAccessUnion `json:"networkAccess"`
	ExcludeSlashTmp     *bool               `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool               `json:"excludeTmpdirEnvVar,omitempty"`
	WritableRoots       []string            `json:"writableRoots,omitempty"`
}

type ThreadShellCommandParams struct {
	// Shell command string evaluated by the thread's configured shell. Unlike `command/exec`,       
	// this intentionally preserves shell syntax such as pipes, redirects, and quoting. This         
	// runs unsandboxed with full access rather than inheriting the thread sandbox policy.           
	Command                                                                                   string `json:"command"`
	ThreadID                                                                                  string `json:"threadId"`
}

type ThreadStartParams struct {
	ApprovalPolicy                                                                         *ThreadStartParamsApprovalPolicy `json:"approvalPolicy"`
	// Override where approval requests are routed for review on this thread and subsequent                                 
	// turns.                                                                                                               
	ApprovalsReviewer                                                                      *ApprovalsReviewer               `json:"approvalsReviewer"`
	BaseInstructions                                                                       *string                          `json:"baseInstructions"`
	Config                                                                                 map[string]interface{}           `json:"config"`
	Cwd                                                                                    *string                          `json:"cwd"`
	DeveloperInstructions                                                                  *string                          `json:"developerInstructions"`
	Ephemeral                                                                              *bool                            `json:"ephemeral"`
	Model                                                                                  *string                          `json:"model"`
	ModelProvider                                                                          *string                          `json:"modelProvider"`
	Personality                                                                            *Personality                     `json:"personality"`
	Sandbox                                                                                *SandboxMode                     `json:"sandbox"`
	ServiceName                                                                            *string                          `json:"serviceName"`
	ServiceTier                                                                            *string                          `json:"serviceTier"`
	SessionStartSource                                                                     *ThreadStartSource               `json:"sessionStartSource"`
	// Optional client-supplied analytics source classification for this thread.                                            
	ThreadSource                                                                           *ThreadSource                    `json:"threadSource"`
}

type HilariousGranularAskForApproval struct {
	Granular AmbitiousGranular `json:"granular"`
}

type AmbitiousGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

type ThreadStartResponse struct {
	ApprovalPolicy                                                                         *ThreadStartResponseAskForApproval `json:"approvalPolicy"`
	// Reviewer currently used for approval requests on this thread.                                                          
	ApprovalsReviewer                                                                      ApprovalsReviewer                  `json:"approvalsReviewer"`
	Cwd                                                                                    string                             `json:"cwd"`
	// Instruction source files currently loaded for this thread.                                                             
	InstructionSources                                                                     []string                           `json:"instructionSources,omitempty"`
	Model                                                                                  string                             `json:"model"`
	ModelProvider                                                                          string                             `json:"modelProvider"`
	ReasoningEffort                                                                        *ReasoningEffort                   `json:"reasoningEffort"`
	// Legacy sandbox policy retained for compatibility. Experimental clients should prefer                                   
	// `activePermissionProfile` for profile provenance.                                                                      
	Sandbox                                                                                ThreadStartResponseSandboxPolicy   `json:"sandbox"`
	ServiceTier                                                                            *string                            `json:"serviceTier"`
	Thread                                                                                 ThreadStartResponseThread          `json:"thread"`
}

type AmbitiousGranularAskForApproval struct {
	Granular CunningGranular `json:"granular"`
}

type CunningGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

// Legacy sandbox policy retained for compatibility. Experimental clients should prefer
// `activePermissionProfile` for profile provenance.
type ThreadStartResponseSandboxPolicy struct {
	Type                SandboxPolicyType   `json:"type"`
	NetworkAccess       *NetworkAccessUnion `json:"networkAccess"`
	ExcludeSlashTmp     *bool               `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool               `json:"excludeTmpdirEnvVar,omitempty"`
	WritableRoots       []string            `json:"writableRoots,omitempty"`
}

type ThreadStartResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                             
	AgentNickname                                                                            *string              `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                  
	AgentRole                                                                                *string              `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                
	CLIVersion                                                                               string               `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                   
	CreatedAt                                                                                int64                `json:"createdAt"`
	// Working directory captured for the thread.                                                                 
	Cwd                                                                                      string               `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                    
	Ephemeral                                                                                bool                 `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                   
	ForkedFromID                                                                             *string              `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                
	GitInfo                                                                                  *IndecentGitInfo     `json:"gitInfo"`
	ID                                                                                       string               `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                               
	ModelProvider                                                                            string               `json:"modelProvider"`
	// Optional user-facing thread title.                                                                         
	Name                                                                                     *string              `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                     
	Path                                                                                     *string              `json:"path"`
	// Usually the first user message in the thread, if available.                                                
	Preview                                                                                  string               `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                         
	SessionID                                                                                string               `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                    
	Source                                                                                   *SessionSource2      `json:"source"`
	// Current runtime status for the thread.                                                                     
	Status                                                                                   IndecentThreadStatus `json:"status"`
	// Optional analytics source classification for this thread.                                                  
	ThreadSource                                                                             *ThreadSource        `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                     
	// (when `includeTurns` is true) responses. For all other responses and notifications                         
	// returning a Thread, the turns field will be an empty list.                                                 
	Turns                                                                                    []IndecentTurn       `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                              
	UpdatedAt                                                                                int64                `json:"updatedAt"`
}

type IndecentGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type HilariousSessionSource struct {
	Custom   *string          `json:"custom,omitempty"`
	SubAgent *SubAgentSource3 `json:"subAgent"`
}

type HilariousSubAgentSource struct {
	ThreadSpawn *HilariousThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string               `json:"other,omitempty"`
}

type HilariousThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type IndecentThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type IndecentTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                       
	CompletedAt                                                             *int64                `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                      
	DurationMS                                                              *int64                `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                           
	Error                                                                   *AmbitiousTurnError   `json:"error"`
	ID                                                                      string                `json:"id"`
	// Thread items currently included in this turn payload.                                      
	Items                                                                   []AmbitiousThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                               
	ItemsView                                                               *TurnItemsView        `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                         
	StartedAt                                                               *int64                `json:"startedAt"`
	Status                                                                  TurnStatus            `json:"status"`
}

type AmbitiousTurnError struct {
	AdditionalDetails *string          `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo9 `json:"codexErrorInfo"`
	Message           string           `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type CunningCodexErrorInfo struct {
	HTTPConnectionFailed           *CunningHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *CunningResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *CunningResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *CunningResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *CunningActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type CunningActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type CunningHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type CunningResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type CunningResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type CunningResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type AmbitiousThreadItem struct {
	Content                                                                                     []MagentaContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                       
	ID                                                                                          string                                    `json:"id"`
	Type                                                                                        ThreadItemType                            `json:"type"`
	Fragments                                                                                   []MagentaHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *MagentaMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                             `json:"phase"`
	Text                                                                                        *string                                   `json:"text,omitempty"`
	Summary                                                                                     []string                                  `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                           
	AggregatedOutput                                                                            *string                                   `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                        
	Command                                                                                     *string                                   `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                             
	// returns a list of CommandAction objects because a single shell command may be composed of                                          
	// many commands piped together.                                                                                                      
	CommandActions                                                                              []MagentaCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                   
	Cwd                                                                                         *string                                   `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                             
	//                                                                                                                                    
	// The duration of the MCP tool call in milliseconds.                                                                                 
	//                                                                                                                                    
	// The duration of the dynamic tool call in milliseconds.                                                                             
	DurationMS                                                                                  *int64                                    `json:"durationMs"`
	// The command's exit code.                                                                                                           
	ExitCode                                                                                    *int64                                    `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                        
	ProcessID                                                                                   *string                                   `json:"processId"`
	Source                                                                                      *CommandExecutionSource                   `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                            
	Status                                                                                      *string                                   `json:"status,omitempty"`
	Changes                                                                                     []MagentaFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                               `json:"arguments"`
	Error                                                                                       *MagentaMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                   `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                   `json:"pluginId"`
	Result                                                                                      *MagentaResult                            `json:"result"`
	Server                                                                                      *string                                   `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                          
	Tool                                                                                        *string                                   `json:"tool,omitempty"`
	ContentItems                                                                                []MagentaDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                   `json:"namespace"`
	Success                                                                                     *bool                                     `json:"success"`
	// Last known status of the target agents, when available.                                                                            
	AgentsStates                                                                                map[string]MagentaCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                            
	Model                                                                                       *string                                   `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                  
	Prompt                                                                                      *string                                   `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                 
	ReasoningEffort                                                                             *ReasoningEffort                          `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                
	// corresponds to the newly spawned agent.                                                                                            
	ReceiverThreadIDS                                                                           []string                                  `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                 
	SenderThreadID                                                                              *string                                   `json:"senderThreadId,omitempty"`
	Action                                                                                      *MagentaWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                   `json:"query,omitempty"`
	Path                                                                                        *string                                   `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                   `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                   `json:"savedPath"`
	Review                                                                                      *string                                   `json:"review,omitempty"`
}

type MagentaWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type MagentaCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type MagentaFileUpdateChange struct {
	Diff string                `json:"diff"`
	Kind FriskyPatchChangeKind `json:"kind"`
	Path string                `json:"path"`
}

type FriskyPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type MagentaCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type MagentaUserInput struct {
	Text                                                                         *string              `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                     
	TextElements                                                                 []MagentaTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType        `json:"type"`
	Detail                                                                       *ImageDetail         `json:"detail"`
	URL                                                                          *string              `json:"url,omitempty"`
	Path                                                                         *string              `json:"path,omitempty"`
	Name                                                                         *string              `json:"name,omitempty"`
}

type MagentaTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                        
	ByteRange                                                                   MagentaByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                 
	Placeholder                                                                 *string          `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type MagentaByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type MagentaDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type MagentaMCPToolCallError struct {
	Message string `json:"message"`
}

type MagentaHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type MagentaMemoryCitation struct {
	Entries   []MagentaMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                     `json:"threadIds"`
}

type MagentaMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type MagentaMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadStartedNotification struct {
	Thread ThreadStartedNotificationThread `json:"thread"`
}

type ThreadStartedNotificationThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                              
	AgentNickname                                                                            *string               `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                   
	AgentRole                                                                                *string               `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                 
	CLIVersion                                                                               string                `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                    
	CreatedAt                                                                                int64                 `json:"createdAt"`
	// Working directory captured for the thread.                                                                  
	Cwd                                                                                      string                `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                     
	Ephemeral                                                                                bool                  `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                    
	ForkedFromID                                                                             *string               `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                 
	GitInfo                                                                                  *HilariousGitInfo     `json:"gitInfo"`
	ID                                                                                       string                `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                                
	ModelProvider                                                                            string                `json:"modelProvider"`
	// Optional user-facing thread title.                                                                          
	Name                                                                                     *string               `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                      
	Path                                                                                     *string               `json:"path"`
	// Usually the first user message in the thread, if available.                                                 
	Preview                                                                                  string                `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                          
	SessionID                                                                                string                `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                     
	Source                                                                                   *SessionSource3       `json:"source"`
	// Current runtime status for the thread.                                                                      
	Status                                                                                   HilariousThreadStatus `json:"status"`
	// Optional analytics source classification for this thread.                                                   
	ThreadSource                                                                             *ThreadSource         `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                      
	// (when `includeTurns` is true) responses. For all other responses and notifications                          
	// returning a Thread, the turns field will be an empty list.                                                  
	Turns                                                                                    []HilariousTurn       `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                               
	UpdatedAt                                                                                int64                 `json:"updatedAt"`
}

type HilariousGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type AmbitiousSessionSource struct {
	Custom   *string          `json:"custom,omitempty"`
	SubAgent *SubAgentSource4 `json:"subAgent"`
}

type AmbitiousSubAgentSource struct {
	ThreadSpawn *AmbitiousThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string               `json:"other,omitempty"`
}

type AmbitiousThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type HilariousThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type HilariousTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                     
	CompletedAt                                                             *int64              `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                    
	DurationMS                                                              *int64              `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                         
	Error                                                                   *CunningTurnError   `json:"error"`
	ID                                                                      string              `json:"id"`
	// Thread items currently included in this turn payload.                                    
	Items                                                                   []CunningThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                             
	ItemsView                                                               *TurnItemsView      `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                       
	StartedAt                                                               *int64              `json:"startedAt"`
	Status                                                                  TurnStatus          `json:"status"`
}

type CunningTurnError struct {
	AdditionalDetails *string           `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo10 `json:"codexErrorInfo"`
	Message           string            `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type MagentaCodexErrorInfo struct {
	HTTPConnectionFailed           *MagentaHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *MagentaResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *MagentaResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *MagentaResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *MagentaActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type MagentaActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type MagentaHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type MagentaResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type MagentaResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type MagentaResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type CunningThreadItem struct {
	Content                                                                                     []FriskyContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                      
	ID                                                                                          string                                   `json:"id"`
	Type                                                                                        ThreadItemType                           `json:"type"`
	Fragments                                                                                   []FriskyHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *FriskyMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                            `json:"phase"`
	Text                                                                                        *string                                  `json:"text,omitempty"`
	Summary                                                                                     []string                                 `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                          
	AggregatedOutput                                                                            *string                                  `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                       
	Command                                                                                     *string                                  `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                            
	// returns a list of CommandAction objects because a single shell command may be composed of                                         
	// many commands piped together.                                                                                                     
	CommandActions                                                                              []FriskyCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                  
	Cwd                                                                                         *string                                  `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                            
	//                                                                                                                                   
	// The duration of the MCP tool call in milliseconds.                                                                                
	//                                                                                                                                   
	// The duration of the dynamic tool call in milliseconds.                                                                            
	DurationMS                                                                                  *int64                                   `json:"durationMs"`
	// The command's exit code.                                                                                                          
	ExitCode                                                                                    *int64                                   `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                       
	ProcessID                                                                                   *string                                  `json:"processId"`
	Source                                                                                      *CommandExecutionSource                  `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                           
	Status                                                                                      *string                                  `json:"status,omitempty"`
	Changes                                                                                     []FriskyFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                              `json:"arguments"`
	Error                                                                                       *FriskyMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                  `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                  `json:"pluginId"`
	Result                                                                                      *FriskyResult                            `json:"result"`
	Server                                                                                      *string                                  `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                         
	Tool                                                                                        *string                                  `json:"tool,omitempty"`
	ContentItems                                                                                []FriskyDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                  `json:"namespace"`
	Success                                                                                     *bool                                    `json:"success"`
	// Last known status of the target agents, when available.                                                                           
	AgentsStates                                                                                map[string]FriskyCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                           
	Model                                                                                       *string                                  `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                 
	Prompt                                                                                      *string                                  `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                
	ReasoningEffort                                                                             *ReasoningEffort                         `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                               
	// corresponds to the newly spawned agent.                                                                                           
	ReceiverThreadIDS                                                                           []string                                 `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                
	SenderThreadID                                                                              *string                                  `json:"senderThreadId,omitempty"`
	Action                                                                                      *FriskyWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                  `json:"query,omitempty"`
	Path                                                                                        *string                                  `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                  `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                  `json:"savedPath"`
	Review                                                                                      *string                                  `json:"review,omitempty"`
}

type FriskyWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type FriskyCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type FriskyFileUpdateChange struct {
	Diff string                     `json:"diff"`
	Kind MischievousPatchChangeKind `json:"kind"`
	Path string                     `json:"path"`
}

type MischievousPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type FriskyCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type FriskyUserInput struct {
	Text                                                                         *string             `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                    
	TextElements                                                                 []FriskyTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType       `json:"type"`
	Detail                                                                       *ImageDetail        `json:"detail"`
	URL                                                                          *string             `json:"url,omitempty"`
	Path                                                                         *string             `json:"path,omitempty"`
	Name                                                                         *string             `json:"name,omitempty"`
}

type FriskyTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                       
	ByteRange                                                                   FriskyByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                
	Placeholder                                                                 *string         `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type FriskyByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type FriskyDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type FriskyMCPToolCallError struct {
	Message string `json:"message"`
}

type FriskyHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type FriskyMemoryCitation struct {
	Entries   []FriskyMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                    `json:"threadIds"`
}

type FriskyMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type FriskyMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadStatusChangedNotification struct {
	Status   ThreadStatusChangedNotificationThreadStatus `json:"status"`
	ThreadID string                                      `json:"threadId"`
}

type ThreadStatusChangedNotificationThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type ThreadTokenUsageUpdatedNotification struct {
	ThreadID   string           `json:"threadId"`
	TokenUsage ThreadTokenUsage `json:"tokenUsage"`
	TurnID     string           `json:"turnId"`
}

type ThreadTokenUsage struct {
	Last               TokenUsageBreakdown `json:"last"`
	ModelContextWindow *int64              `json:"modelContextWindow"`
	Total              TokenUsageBreakdown `json:"total"`
}

type TokenUsageBreakdown struct {
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	InputTokens           int64 `json:"inputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	TotalTokens           int64 `json:"totalTokens"`
}

type ThreadUnarchiveParams struct {
	ThreadID string `json:"threadId"`
}

type ThreadUnarchiveResponse struct {
	Thread ThreadUnarchiveResponseThread `json:"thread"`
}

type ThreadUnarchiveResponseThread struct {
	// Optional random unique nickname assigned to an AgentControl-spawned sub-agent.                              
	AgentNickname                                                                            *string               `json:"agentNickname"`
	// Optional role (agent_role) assigned to an AgentControl-spawned sub-agent.                                   
	AgentRole                                                                                *string               `json:"agentRole"`
	// Version of the CLI that created the thread.                                                                 
	CLIVersion                                                                               string                `json:"cliVersion"`
	// Unix timestamp (in seconds) when the thread was created.                                                    
	CreatedAt                                                                                int64                 `json:"createdAt"`
	// Working directory captured for the thread.                                                                  
	Cwd                                                                                      string                `json:"cwd"`
	// Whether the thread is ephemeral and should not be materialized on disk.                                     
	Ephemeral                                                                                bool                  `json:"ephemeral"`
	// Source thread id when this thread was created by forking another thread.                                    
	ForkedFromID                                                                             *string               `json:"forkedFromId"`
	// Optional Git metadata captured when the thread was created.                                                 
	GitInfo                                                                                  *AmbitiousGitInfo     `json:"gitInfo"`
	ID                                                                                       string                `json:"id"`
	// Model provider used for this thread (for example, 'openai').                                                
	ModelProvider                                                                            string                `json:"modelProvider"`
	// Optional user-facing thread title.                                                                          
	Name                                                                                     *string               `json:"name"`
	// [UNSTABLE] Path to the thread on disk.                                                                      
	Path                                                                                     *string               `json:"path"`
	// Usually the first user message in the thread, if available.                                                 
	Preview                                                                                  string                `json:"preview"`
	// Session id shared by threads that belong to the same session tree.                                          
	SessionID                                                                                string                `json:"sessionId"`
	// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).                                     
	Source                                                                                   *SessionSource4       `json:"source"`
	// Current runtime status for the thread.                                                                      
	Status                                                                                   AmbitiousThreadStatus `json:"status"`
	// Optional analytics source classification for this thread.                                                   
	ThreadSource                                                                             *ThreadSource         `json:"threadSource"`
	// Only populated on `thread/resume`, `thread/rollback`, `thread/fork`, and `thread/read`                      
	// (when `includeTurns` is true) responses. For all other responses and notifications                          
	// returning a Thread, the turns field will be an empty list.                                                  
	Turns                                                                                    []AmbitiousTurn       `json:"turns"`
	// Unix timestamp (in seconds) when the thread was last updated.                                               
	UpdatedAt                                                                                int64                 `json:"updatedAt"`
}

type AmbitiousGitInfo struct {
	Branch    *string `json:"branch"`
	OriginURL *string `json:"originUrl"`
	SHA       *string `json:"sha"`
}

type CunningSessionSource struct {
	Custom   *string          `json:"custom,omitempty"`
	SubAgent *SubAgentSource5 `json:"subAgent"`
}

type CunningSubAgentSource struct {
	ThreadSpawn *CunningThreadSpawn `json:"thread_spawn,omitempty"`
	Other       *string             `json:"other,omitempty"`
}

type CunningThreadSpawn struct {
	AgentNickname  *string `json:"agent_nickname"`
	AgentPath      *string `json:"agent_path"`
	AgentRole      *string `json:"agent_role"`
	Depth          int64   `json:"depth"`
	ParentThreadID string  `json:"parent_thread_id"`
}

// Current runtime status for the thread.
type AmbitiousThreadStatus struct {
	Type        ThreadStatusType   `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags,omitempty"`
}

type AmbitiousTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                     
	CompletedAt                                                             *int64              `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                    
	DurationMS                                                              *int64              `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                         
	Error                                                                   *MagentaTurnError   `json:"error"`
	ID                                                                      string              `json:"id"`
	// Thread items currently included in this turn payload.                                    
	Items                                                                   []MagentaThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                             
	ItemsView                                                               *TurnItemsView      `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                       
	StartedAt                                                               *int64              `json:"startedAt"`
	Status                                                                  TurnStatus          `json:"status"`
}

type MagentaTurnError struct {
	AdditionalDetails *string           `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo11 `json:"codexErrorInfo"`
	Message           string            `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type FriskyCodexErrorInfo struct {
	HTTPConnectionFailed           *FriskyHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *FriskyResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *FriskyResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *FriskyResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *FriskyActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type FriskyActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type FriskyHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type FriskyResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type FriskyResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type FriskyResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type MagentaThreadItem struct {
	Content                                                                                     []MischievousContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                           
	ID                                                                                          string                                        `json:"id"`
	Type                                                                                        ThreadItemType                                `json:"type"`
	Fragments                                                                                   []MischievousHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *MischievousMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                                 `json:"phase"`
	Text                                                                                        *string                                       `json:"text,omitempty"`
	Summary                                                                                     []string                                      `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                               
	AggregatedOutput                                                                            *string                                       `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                            
	Command                                                                                     *string                                       `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                                 
	// returns a list of CommandAction objects because a single shell command may be composed of                                              
	// many commands piped together.                                                                                                          
	CommandActions                                                                              []MischievousCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                       
	Cwd                                                                                         *string                                       `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                                 
	//                                                                                                                                        
	// The duration of the MCP tool call in milliseconds.                                                                                     
	//                                                                                                                                        
	// The duration of the dynamic tool call in milliseconds.                                                                                 
	DurationMS                                                                                  *int64                                        `json:"durationMs"`
	// The command's exit code.                                                                                                               
	ExitCode                                                                                    *int64                                        `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                            
	ProcessID                                                                                   *string                                       `json:"processId"`
	Source                                                                                      *CommandExecutionSource                       `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                                
	Status                                                                                      *string                                       `json:"status,omitempty"`
	Changes                                                                                     []MischievousFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                                   `json:"arguments"`
	Error                                                                                       *MischievousMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                       `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                       `json:"pluginId"`
	Result                                                                                      *MischievousResult                            `json:"result"`
	Server                                                                                      *string                                       `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                              
	Tool                                                                                        *string                                       `json:"tool,omitempty"`
	ContentItems                                                                                []MischievousDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                       `json:"namespace"`
	Success                                                                                     *bool                                         `json:"success"`
	// Last known status of the target agents, when available.                                                                                
	AgentsStates                                                                                map[string]MischievousCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                                
	Model                                                                                       *string                                       `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                      
	Prompt                                                                                      *string                                       `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                     
	ReasoningEffort                                                                             *ReasoningEffort                              `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                    
	// corresponds to the newly spawned agent.                                                                                                
	ReceiverThreadIDS                                                                           []string                                      `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                     
	SenderThreadID                                                                              *string                                       `json:"senderThreadId,omitempty"`
	Action                                                                                      *MischievousWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                       `json:"query,omitempty"`
	Path                                                                                        *string                                       `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                       `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                       `json:"savedPath"`
	Review                                                                                      *string                                       `json:"review,omitempty"`
}

type MischievousWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type MischievousCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type MischievousFileUpdateChange struct {
	Diff string                       `json:"diff"`
	Kind BraggadociousPatchChangeKind `json:"kind"`
	Path string                       `json:"path"`
}

type BraggadociousPatchChangeKind struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type MischievousCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type MischievousUserInput struct {
	Text                                                                         *string                  `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                         
	TextElements                                                                 []MischievousTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType            `json:"type"`
	Detail                                                                       *ImageDetail             `json:"detail"`
	URL                                                                          *string                  `json:"url,omitempty"`
	Path                                                                         *string                  `json:"path,omitempty"`
	Name                                                                         *string                  `json:"name,omitempty"`
}

type MischievousTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                            
	ByteRange                                                                   MischievousByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                     
	Placeholder                                                                 *string              `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type MischievousByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type MischievousDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type MischievousMCPToolCallError struct {
	Message string `json:"message"`
}

type MischievousHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type MischievousMemoryCitation struct {
	Entries   []MischievousMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                         `json:"threadIds"`
}

type MischievousMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type MischievousMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type ThreadUnarchivedNotification struct {
	ThreadID string `json:"threadId"`
}

type ThreadUnsubscribeParams struct {
	ThreadID string `json:"threadId"`
}

type ThreadUnsubscribeResponse struct {
	Status ThreadUnsubscribeStatus `json:"status"`
}

type TurnCompletedNotification struct {
	ThreadID string                        `json:"threadId"`
	Turn     TurnCompletedNotificationTurn `json:"turn"`
}

type TurnCompletedNotificationTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                    
	CompletedAt                                                             *int64             `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                   
	DurationMS                                                              *int64             `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                        
	Error                                                                   *FriskyTurnError   `json:"error"`
	ID                                                                      string             `json:"id"`
	// Thread items currently included in this turn payload.                                   
	Items                                                                   []FriskyThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                            
	ItemsView                                                               *TurnItemsView     `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                      
	StartedAt                                                               *int64             `json:"startedAt"`
	Status                                                                  TurnStatus         `json:"status"`
}

type FriskyTurnError struct {
	AdditionalDetails *string           `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo12 `json:"codexErrorInfo"`
	Message           string            `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type MischievousCodexErrorInfo struct {
	HTTPConnectionFailed           *MischievousHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *MischievousResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *MischievousResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *MischievousResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *MischievousActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type MischievousActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type MischievousHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type MischievousResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type MischievousResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type MischievousResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type FriskyThreadItem struct {
	Content                                                                                     []BraggadociousContent                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                             
	ID                                                                                          string                                          `json:"id"`
	Type                                                                                        ThreadItemType                                  `json:"type"`
	Fragments                                                                                   []BraggadociousHookPromptFragment               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *BraggadociousMemoryCitation                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                                   `json:"phase"`
	Text                                                                                        *string                                         `json:"text,omitempty"`
	Summary                                                                                     []string                                        `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                                 
	AggregatedOutput                                                                            *string                                         `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                              
	Command                                                                                     *string                                         `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                                   
	// returns a list of CommandAction objects because a single shell command may be composed of                                                
	// many commands piped together.                                                                                                            
	CommandActions                                                                              []BraggadociousCommandAction                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                                         
	Cwd                                                                                         *string                                         `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                                   
	//                                                                                                                                          
	// The duration of the MCP tool call in milliseconds.                                                                                       
	//                                                                                                                                          
	// The duration of the dynamic tool call in milliseconds.                                                                                   
	DurationMS                                                                                  *int64                                          `json:"durationMs"`
	// The command's exit code.                                                                                                                 
	ExitCode                                                                                    *int64                                          `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                              
	ProcessID                                                                                   *string                                         `json:"processId"`
	Source                                                                                      *CommandExecutionSource                         `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                                  
	Status                                                                                      *string                                         `json:"status,omitempty"`
	Changes                                                                                     []BraggadociousFileUpdateChange                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                                     `json:"arguments"`
	Error                                                                                       *BraggadociousMCPToolCallError                  `json:"error"`
	MCPAppResourceURI                                                                           *string                                         `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                                         `json:"pluginId"`
	Result                                                                                      *BraggadociousResult                            `json:"result"`
	Server                                                                                      *string                                         `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                                
	Tool                                                                                        *string                                         `json:"tool,omitempty"`
	ContentItems                                                                                []BraggadociousDynamicToolCallOutputContentItem `json:"contentItems"`
	Namespace                                                                                   *string                                         `json:"namespace"`
	Success                                                                                     *bool                                           `json:"success"`
	// Last known status of the target agents, when available.                                                                                  
	AgentsStates                                                                                map[string]BraggadociousCollabAgentState        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                                  
	Model                                                                                       *string                                         `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                                        
	Prompt                                                                                      *string                                         `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                                       
	ReasoningEffort                                                                             *ReasoningEffort                                `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                                      
	// corresponds to the newly spawned agent.                                                                                                  
	ReceiverThreadIDS                                                                           []string                                        `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                                       
	SenderThreadID                                                                              *string                                         `json:"senderThreadId,omitempty"`
	Action                                                                                      *BraggadociousWebSearchAction                   `json:"action"`
	Query                                                                                       *string                                         `json:"query,omitempty"`
	Path                                                                                        *string                                         `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                                         `json:"revisedPrompt"`
	SavedPath                                                                                   *string                                         `json:"savedPath"`
	Review                                                                                      *string                                         `json:"review,omitempty"`
}

type BraggadociousWebSearchAction struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type BraggadociousCollabAgentState struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type BraggadociousFileUpdateChange struct {
	Diff string           `json:"diff"`
	Kind PatchChangeKind1 `json:"kind"`
	Path string           `json:"path"`
}

type PatchChangeKind1 struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type BraggadociousCommandAction struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type BraggadociousUserInput struct {
	Text                                                                         *string                    `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.                           
	TextElements                                                                 []BraggadociousTextElement `json:"text_elements,omitempty"`
	Type                                                                         UserInputType              `json:"type"`
	Detail                                                                       *ImageDetail               `json:"detail"`
	URL                                                                          *string                    `json:"url,omitempty"`
	Path                                                                         *string                    `json:"path,omitempty"`
	Name                                                                         *string                    `json:"name,omitempty"`
}

type BraggadociousTextElement struct {
	// Byte range in the parent `text` buffer that this element occupies.                              
	ByteRange                                                                   BraggadociousByteRange `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.                       
	Placeholder                                                                 *string                `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type BraggadociousByteRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type BraggadociousDynamicToolCallOutputContentItem struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type BraggadociousMCPToolCallError struct {
	Message string `json:"message"`
}

type BraggadociousHookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type BraggadociousMemoryCitation struct {
	Entries   []BraggadociousMemoryCitationEntry `json:"entries"`
	ThreadIDS []string                           `json:"threadIds"`
}

type BraggadociousMemoryCitationEntry struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type BraggadociousMCPToolCallResult struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

// Notification that the turn-level unified diff has changed. Contains the latest aggregated
// diff across all file changes in the turn.
type TurnDiffUpdatedNotification struct {
	Diff     string `json:"diff"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type TurnInterruptParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type TurnPlanUpdatedNotification struct {
	Explanation *string        `json:"explanation"`
	Plan        []TurnPlanStep `json:"plan"`
	ThreadID    string         `json:"threadId"`
	TurnID      string         `json:"turnId"`
}

type TurnPlanStep struct {
	Status TurnPlanStepStatus `json:"status"`
	Step   string             `json:"step"`
}

type TurnStartParams struct {
	// Override the approval policy for this turn and subsequent turns.                                                                       
	ApprovalPolicy                                                                              *TurnStartParamsApprovalPolicy                `json:"approvalPolicy"`
	// Override where approval requests are routed for review on this turn and subsequent turns.                                              
	ApprovalsReviewer                                                                           *ApprovalsReviewer                            `json:"approvalsReviewer"`
	// Override the working directory for this turn and subsequent turns.                                                                     
	Cwd                                                                                         *string                                       `json:"cwd"`
	// Override the reasoning effort for this turn and subsequent turns.                                                                      
	Effort                                                                                      *ReasoningEffort                              `json:"effort"`
	Input                                                                                       []TurnStartParamsUserInput                    `json:"input"`
	// Override the model for this turn and subsequent turns.                                                                                 
	Model                                                                                       *string                                       `json:"model"`
	// Optional JSON Schema used to constrain the final assistant message for this turn.                                                      
	OutputSchema                                                                                interface{}                                   `json:"outputSchema"`
	// Override the personality for this turn and subsequent turns.                                                                           
	Personality                                                                                 *Personality                                  `json:"personality"`
	// Override the sandbox policy for this turn and subsequent turns.                                                                        
	SandboxPolicy                                                                               *TurnStartParamsDangerFullAccessSandboxPolicy `json:"sandboxPolicy"`
	// Override the service tier for this turn and subsequent turns.                                                                          
	ServiceTier                                                                                 *string                                       `json:"serviceTier"`
	// Override the reasoning summary for this turn and subsequent turns.                                                                     
	Summary                                                                                     *ReasoningSummary                             `json:"summary"`
	ThreadID                                                                                    string                                        `json:"threadId"`
}

type CunningGranularAskForApproval struct {
	Granular MagentaGranular `json:"granular"`
}

type MagentaGranular struct {
	MCPElicitations    bool  `json:"mcp_elicitations"`
	RequestPermissions *bool `json:"request_permissions,omitempty"`
	Rules              bool  `json:"rules"`
	SandboxApproval    bool  `json:"sandbox_approval"`
	SkillApproval      *bool `json:"skill_approval,omitempty"`
}

type TurnStartParamsUserInput struct {
	Text                                                                         *string        `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.               
	TextElements                                                                 []TextElement1 `json:"text_elements,omitempty"`
	Type                                                                         UserInputType  `json:"type"`
	Detail                                                                       *ImageDetail   `json:"detail"`
	URL                                                                          *string        `json:"url,omitempty"`
	Path                                                                         *string        `json:"path,omitempty"`
	Name                                                                         *string        `json:"name,omitempty"`
}

type TextElement1 struct {
	// Byte range in the parent `text` buffer that this element occupies.                  
	ByteRange                                                                   ByteRange1 `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.           
	Placeholder                                                                 *string    `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type ByteRange1 struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type TurnStartParamsDangerFullAccessSandboxPolicy struct {
	Type                SandboxPolicyType   `json:"type"`
	NetworkAccess       *NetworkAccessUnion `json:"networkAccess"`
	ExcludeSlashTmp     *bool               `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool               `json:"excludeTmpdirEnvVar,omitempty"`
	WritableRoots       []string            `json:"writableRoots,omitempty"`
}

type TurnStartResponse struct {
	Turn TurnStartResponseTurn `json:"turn"`
}

type TurnStartResponseTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                         
	CompletedAt                                                             *int64                  `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                        
	DurationMS                                                              *int64                  `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                             
	Error                                                                   *MischievousTurnError   `json:"error"`
	ID                                                                      string                  `json:"id"`
	// Thread items currently included in this turn payload.                                        
	Items                                                                   []MischievousThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                                 
	ItemsView                                                               *TurnItemsView          `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                           
	StartedAt                                                               *int64                  `json:"startedAt"`
	Status                                                                  TurnStatus              `json:"status"`
}

type MischievousTurnError struct {
	AdditionalDetails *string           `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo13 `json:"codexErrorInfo"`
	Message           string            `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type BraggadociousCodexErrorInfo struct {
	HTTPConnectionFailed           *BraggadociousHTTPConnectionFailed           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *BraggadociousResponseStreamConnectionFailed `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *BraggadociousResponseStreamDisconnected     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *BraggadociousResponseTooManyFailedAttempts  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *BraggadociousActiveTurnNotSteerable         `json:"activeTurnNotSteerable,omitempty"`
}

type BraggadociousActiveTurnNotSteerable struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type BraggadociousHTTPConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type BraggadociousResponseStreamConnectionFailed struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type BraggadociousResponseStreamDisconnected struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type BraggadociousResponseTooManyFailedAttempts struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type MischievousThreadItem struct {
	Content                                                                                     []Content1                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                 
	ID                                                                                          string                              `json:"id"`
	Type                                                                                        ThreadItemType                      `json:"type"`
	Fragments                                                                                   []HookPromptFragment1               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *MemoryCitation1                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                       `json:"phase"`
	Text                                                                                        *string                             `json:"text,omitempty"`
	Summary                                                                                     []string                            `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                     
	AggregatedOutput                                                                            *string                             `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                  
	Command                                                                                     *string                             `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                       
	// returns a list of CommandAction objects because a single shell command may be composed of                                    
	// many commands piped together.                                                                                                
	CommandActions                                                                              []CommandAction1                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                             
	Cwd                                                                                         *string                             `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                       
	//                                                                                                                              
	// The duration of the MCP tool call in milliseconds.                                                                           
	//                                                                                                                              
	// The duration of the dynamic tool call in milliseconds.                                                                       
	DurationMS                                                                                  *int64                              `json:"durationMs"`
	// The command's exit code.                                                                                                     
	ExitCode                                                                                    *int64                              `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                  
	ProcessID                                                                                   *string                             `json:"processId"`
	Source                                                                                      *CommandExecutionSource             `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                      
	Status                                                                                      *string                             `json:"status,omitempty"`
	Changes                                                                                     []FileUpdateChange1                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                         `json:"arguments"`
	Error                                                                                       *MCPToolCallError1                  `json:"error"`
	MCPAppResourceURI                                                                           *string                             `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                             `json:"pluginId"`
	Result                                                                                      *Result1                            `json:"result"`
	Server                                                                                      *string                             `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                    
	Tool                                                                                        *string                             `json:"tool,omitempty"`
	ContentItems                                                                                []DynamicToolCallOutputContentItem1 `json:"contentItems"`
	Namespace                                                                                   *string                             `json:"namespace"`
	Success                                                                                     *bool                               `json:"success"`
	// Last known status of the target agents, when available.                                                                      
	AgentsStates                                                                                map[string]CollabAgentState1        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                      
	Model                                                                                       *string                             `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                            
	Prompt                                                                                      *string                             `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                           
	ReasoningEffort                                                                             *ReasoningEffort                    `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                          
	// corresponds to the newly spawned agent.                                                                                      
	ReceiverThreadIDS                                                                           []string                            `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                           
	SenderThreadID                                                                              *string                             `json:"senderThreadId,omitempty"`
	Action                                                                                      *WebSearchAction1                   `json:"action"`
	Query                                                                                       *string                             `json:"query,omitempty"`
	Path                                                                                        *string                             `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                             `json:"revisedPrompt"`
	SavedPath                                                                                   *string                             `json:"savedPath"`
	Review                                                                                      *string                             `json:"review,omitempty"`
}

type WebSearchAction1 struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type CollabAgentState1 struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type FileUpdateChange1 struct {
	Diff string           `json:"diff"`
	Kind PatchChangeKind2 `json:"kind"`
	Path string           `json:"path"`
}

type PatchChangeKind2 struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type CommandAction1 struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type UserInput1 struct {
	Text                                                                         *string        `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.               
	TextElements                                                                 []TextElement2 `json:"text_elements,omitempty"`
	Type                                                                         UserInputType  `json:"type"`
	Detail                                                                       *ImageDetail   `json:"detail"`
	URL                                                                          *string        `json:"url,omitempty"`
	Path                                                                         *string        `json:"path,omitempty"`
	Name                                                                         *string        `json:"name,omitempty"`
}

type TextElement2 struct {
	// Byte range in the parent `text` buffer that this element occupies.                  
	ByteRange                                                                   ByteRange2 `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.           
	Placeholder                                                                 *string    `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type ByteRange2 struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type DynamicToolCallOutputContentItem1 struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type MCPToolCallError1 struct {
	Message string `json:"message"`
}

type HookPromptFragment1 struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type MemoryCitation1 struct {
	Entries   []MemoryCitationEntry1 `json:"entries"`
	ThreadIDS []string               `json:"threadIds"`
}

type MemoryCitationEntry1 struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type MCPToolCallResult1 struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type TurnStartedNotification struct {
	ThreadID string                      `json:"threadId"`
	Turn     TurnStartedNotificationTurn `json:"turn"`
}

type TurnStartedNotificationTurn struct {
	// Unix timestamp (in seconds) when the turn completed.                                           
	CompletedAt                                                             *int64                    `json:"completedAt"`
	// Duration between turn start and completion in milliseconds, if known.                          
	DurationMS                                                              *int64                    `json:"durationMs"`
	// Only populated when the Turn's status is failed.                                               
	Error                                                                   *BraggadociousTurnError   `json:"error"`
	ID                                                                      string                    `json:"id"`
	// Thread items currently included in this turn payload.                                          
	Items                                                                   []BraggadociousThreadItem `json:"items"`
	// Describes how much of `items` has been loaded for this turn.                                   
	ItemsView                                                               *TurnItemsView            `json:"itemsView,omitempty"`
	// Unix timestamp (in seconds) when the turn started.                                             
	StartedAt                                                               *int64                    `json:"startedAt"`
	Status                                                                  TurnStatus                `json:"status"`
}

type BraggadociousTurnError struct {
	AdditionalDetails *string           `json:"additionalDetails"`
	CodexErrorInfo    *CodexErrorInfo14 `json:"codexErrorInfo"`
	Message           string            `json:"message"`
}

// Failed to connect to the response SSE stream.
//
// The response SSE stream disconnected in the middle of a turn before completion.
//
// Reached the retry limit for responses.
//
// Returned when `turn/start` or `turn/steer` is submitted while the current active turn
// cannot accept same-turn steering, for example `/review` or manual `/compact`.
type CodexErrorInfo1 struct {
	HTTPConnectionFailed           *HTTPConnectionFailed1           `json:"httpConnectionFailed,omitempty"`
	ResponseStreamConnectionFailed *ResponseStreamConnectionFailed1 `json:"responseStreamConnectionFailed,omitempty"`
	ResponseStreamDisconnected     *ResponseStreamDisconnected1     `json:"responseStreamDisconnected,omitempty"`
	ResponseTooManyFailedAttempts  *ResponseTooManyFailedAttempts1  `json:"responseTooManyFailedAttempts,omitempty"`
	ActiveTurnNotSteerable         *ActiveTurnNotSteerable1         `json:"activeTurnNotSteerable,omitempty"`
}

type ActiveTurnNotSteerable1 struct {
	TurnKind NonSteerableTurnKind `json:"turnKind"`
}

type HTTPConnectionFailed1 struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type ResponseStreamConnectionFailed1 struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type ResponseStreamDisconnected1 struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

type ResponseTooManyFailedAttempts1 struct {
	HTTPStatusCode *int64 `json:"httpStatusCode"`
}

// EXPERIMENTAL - proposed plan item content. The completed plan item is authoritative and
// may not match the concatenation of `PlanDelta` text.
type BraggadociousThreadItem struct {
	Content                                                                                     []Content2                          `json:"content,omitempty"`
	// Unique identifier for this collab tool call.                                                                                 
	ID                                                                                          string                              `json:"id"`
	Type                                                                                        ThreadItemType                      `json:"type"`
	Fragments                                                                                   []HookPromptFragment2               `json:"fragments,omitempty"`
	MemoryCitation                                                                              *MemoryCitation2                    `json:"memoryCitation"`
	Phase                                                                                       *MessagePhase                       `json:"phase"`
	Text                                                                                        *string                             `json:"text,omitempty"`
	Summary                                                                                     []string                            `json:"summary,omitempty"`
	// The command's output, aggregated from stdout and stderr.                                                                     
	AggregatedOutput                                                                            *string                             `json:"aggregatedOutput"`
	// The command to be executed.                                                                                                  
	Command                                                                                     *string                             `json:"command,omitempty"`
	// A best-effort parsing of the command to understand the action(s) it will perform. This                                       
	// returns a list of CommandAction objects because a single shell command may be composed of                                    
	// many commands piped together.                                                                                                
	CommandActions                                                                              []CommandAction2                    `json:"commandActions,omitempty"`
	// The command's working directory.                                                                                             
	Cwd                                                                                         *string                             `json:"cwd,omitempty"`
	// The duration of the command execution in milliseconds.                                                                       
	//                                                                                                                              
	// The duration of the MCP tool call in milliseconds.                                                                           
	//                                                                                                                              
	// The duration of the dynamic tool call in milliseconds.                                                                       
	DurationMS                                                                                  *int64                              `json:"durationMs"`
	// The command's exit code.                                                                                                     
	ExitCode                                                                                    *int64                              `json:"exitCode"`
	// Identifier for the underlying PTY process (when available).                                                                  
	ProcessID                                                                                   *string                             `json:"processId"`
	Source                                                                                      *CommandExecutionSource             `json:"source,omitempty"`
	// Current status of the collab tool call.                                                                                      
	Status                                                                                      *string                             `json:"status,omitempty"`
	Changes                                                                                     []FileUpdateChange2                 `json:"changes,omitempty"`
	Arguments                                                                                   interface{}                         `json:"arguments"`
	Error                                                                                       *MCPToolCallError2                  `json:"error"`
	MCPAppResourceURI                                                                           *string                             `json:"mcpAppResourceUri"`
	PluginID                                                                                    *string                             `json:"pluginId"`
	Result                                                                                      *Result2                            `json:"result"`
	Server                                                                                      *string                             `json:"server,omitempty"`
	// Name of the collab tool that was invoked.                                                                                    
	Tool                                                                                        *string                             `json:"tool,omitempty"`
	ContentItems                                                                                []DynamicToolCallOutputContentItem2 `json:"contentItems"`
	Namespace                                                                                   *string                             `json:"namespace"`
	Success                                                                                     *bool                               `json:"success"`
	// Last known status of the target agents, when available.                                                                      
	AgentsStates                                                                                map[string]CollabAgentState2        `json:"agentsStates,omitempty"`
	// Model requested for the spawned agent, when applicable.                                                                      
	Model                                                                                       *string                             `json:"model"`
	// Prompt text sent as part of the collab tool call, when available.                                                            
	Prompt                                                                                      *string                             `json:"prompt"`
	// Reasoning effort requested for the spawned agent, when applicable.                                                           
	ReasoningEffort                                                                             *ReasoningEffort                    `json:"reasoningEffort"`
	// Thread ID of the receiving agent, when applicable. In case of spawn operation, this                                          
	// corresponds to the newly spawned agent.                                                                                      
	ReceiverThreadIDS                                                                           []string                            `json:"receiverThreadIds,omitempty"`
	// Thread ID of the agent issuing the collab request.                                                                           
	SenderThreadID                                                                              *string                             `json:"senderThreadId,omitempty"`
	Action                                                                                      *WebSearchAction2                   `json:"action"`
	Query                                                                                       *string                             `json:"query,omitempty"`
	Path                                                                                        *string                             `json:"path,omitempty"`
	RevisedPrompt                                                                               *string                             `json:"revisedPrompt"`
	SavedPath                                                                                   *string                             `json:"savedPath"`
	Review                                                                                      *string                             `json:"review,omitempty"`
}

type WebSearchAction2 struct {
	Queries []string            `json:"queries"`
	Query   *string             `json:"query"`
	Type    WebSearchActionType `json:"type"`
	URL     *string             `json:"url"`
	Pattern *string             `json:"pattern"`
}

type CollabAgentState2 struct {
	Message *string           `json:"message"`
	Status  CollabAgentStatus `json:"status"`
}

type FileUpdateChange2 struct {
	Diff string           `json:"diff"`
	Kind PatchChangeKind3 `json:"kind"`
	Path string           `json:"path"`
}

type PatchChangeKind3 struct {
	Type     PatchChangeKindType `json:"type"`
	MovePath *string             `json:"move_path"`
}

type CommandAction2 struct {
	Command string            `json:"command"`
	Name    *string           `json:"name,omitempty"`
	Path    *string           `json:"path"`
	Type    CommandActionType `json:"type"`
	Query   *string           `json:"query"`
}

type UserInput2 struct {
	Text                                                                         *string        `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.               
	TextElements                                                                 []TextElement3 `json:"text_elements,omitempty"`
	Type                                                                         UserInputType  `json:"type"`
	Detail                                                                       *ImageDetail   `json:"detail"`
	URL                                                                          *string        `json:"url,omitempty"`
	Path                                                                         *string        `json:"path,omitempty"`
	Name                                                                         *string        `json:"name,omitempty"`
}

type TextElement3 struct {
	// Byte range in the parent `text` buffer that this element occupies.                  
	ByteRange                                                                   ByteRange3 `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.           
	Placeholder                                                                 *string    `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type ByteRange3 struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type DynamicToolCallOutputContentItem2 struct {
	Text     *string                                   `json:"text,omitempty"`
	Type     InputDynamicToolCallOutputContentItemType `json:"type"`
	ImageURL *string                                   `json:"imageUrl,omitempty"`
}

type MCPToolCallError2 struct {
	Message string `json:"message"`
}

type HookPromptFragment2 struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

type MemoryCitation2 struct {
	Entries   []MemoryCitationEntry2 `json:"entries"`
	ThreadIDS []string               `json:"threadIds"`
}

type MemoryCitationEntry2 struct {
	LineEnd   int64  `json:"lineEnd"`
	LineStart int64  `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type MCPToolCallResult2 struct {
	Meta              interface{}   `json:"_meta"`
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent"`
}

type TurnSteerParams struct {
	// Required active turn id precondition. The request fails when it does not match the                           
	// currently active turn.                                                                                       
	ExpectedTurnID                                                                       string                     `json:"expectedTurnId"`
	Input                                                                                []TurnSteerParamsUserInput `json:"input"`
	ThreadID                                                                             string                     `json:"threadId"`
}

type TurnSteerParamsUserInput struct {
	Text                                                                         *string        `json:"text,omitempty"`
	// UI-defined spans within `text` used to render or persist special elements.               
	TextElements                                                                 []TextElement4 `json:"text_elements,omitempty"`
	Type                                                                         UserInputType  `json:"type"`
	Detail                                                                       *ImageDetail   `json:"detail"`
	URL                                                                          *string        `json:"url,omitempty"`
	Path                                                                         *string        `json:"path,omitempty"`
	Name                                                                         *string        `json:"name,omitempty"`
}

type TextElement4 struct {
	// Byte range in the parent `text` buffer that this element occupies.                  
	ByteRange                                                                   ByteRange4 `json:"byteRange"`
	// Optional human-readable placeholder for the element, displayed in the UI.           
	Placeholder                                                                 *string    `json:"placeholder"`
}

// Byte range in the parent `text` buffer that this element occupies.
type ByteRange4 struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

type TurnSteerResponse struct {
	TurnID string `json:"turnId"`
}

type WarningNotification struct {
	// Concise warning message for the user.                                        
	Message                                                                 string  `json:"message"`
	// Optional thread target when the warning applies to a specific thread.        
	ThreadID                                                                *string `json:"threadId"`
}

type WindowsSandboxReadinessResponse struct {
	Status WindowsSandboxReadiness `json:"status"`
}

type WindowsSandboxSetupCompletedNotification struct {
	Error   *string                 `json:"error"`
	Mode    WindowsSandboxSetupMode `json:"mode"`
	Success bool                    `json:"success"`
}

type WindowsSandboxSetupStartParams struct {
	Cwd  *string                 `json:"cwd"`
	Mode WindowsSandboxSetupMode `json:"mode"`
}

type WindowsSandboxSetupStartResponse struct {
	Started bool `json:"started"`
}

type WindowsWorldWritableWarningNotification struct {
	ExtraCount  int64    `json:"extraCount"`
	FailedScan  bool     `json:"failedScan"`
	SamplePaths []string `json:"samplePaths"`
}

type PlanType string

const (
	Business                    PlanType = "business"
	Edu                         PlanType = "edu"
	Enterprise                  PlanType = "enterprise"
	EnterpriseCbpUsageBased     PlanType = "enterprise_cbp_usage_based"
	Free                        PlanType = "free"
	Go                          PlanType = "go"
	PlanTypeUnknown             PlanType = "unknown"
	Plus                        PlanType = "plus"
	Pro                         PlanType = "pro"
	Prolite                     PlanType = "prolite"
	SelfServeBusinessUsageBased PlanType = "self_serve_business_usage_based"
	Team                        PlanType = "team"
)

type RateLimitReachedType string

const (
	RateLimitReached                 RateLimitReachedType = "rate_limit_reached"
	WorkspaceMemberCreditsDepleted   RateLimitReachedType = "workspace_member_credits_depleted"
	WorkspaceMemberUsageLimitReached RateLimitReachedType = "workspace_member_usage_limit_reached"
	WorkspaceOwnerCreditsDepleted    RateLimitReachedType = "workspace_owner_credits_depleted"
	WorkspaceOwnerUsageLimitReached  RateLimitReachedType = "workspace_owner_usage_limit_reached"
)

// OpenAI API key provided by the caller and stored by Codex.
//
// ChatGPT OAuth managed by Codex (tokens persisted and refreshed by Codex).
//
// [UNSTABLE] FOR OPENAI INTERNAL USE ONLY - DO NOT USE.
//
// ChatGPT auth tokens are supplied by an external host app and are only stored in memory.
// Token refresh must be handled by the external host app.
//
// Programmatic Codex auth backed by a registered Agent Identity.
type AuthMode string

const (
	AgentIdentity             AuthMode = "agentIdentity"
	Apikey                    AuthMode = "apikey"
	AuthModeChatgpt           AuthMode = "chatgpt"
	AuthModeChatgptAuthTokens AuthMode = "chatgptAuthTokens"
)

type CancelLoginAccountStatus string

const (
	CancelLoginAccountStatusNotFound CancelLoginAccountStatus = "notFound"
	Canceled                         CancelLoginAccountStatus = "canceled"
)

// Output stream for this chunk.
//
// Stream label for `command/exec/outputDelta` notifications.
//
// stdout stream. PTY mode multiplexes terminal output here.
//
// stderr stream.
//
// Output stream this chunk belongs to.
//
// Stream label for `process/outputDelta` notifications.
type OutputStream string

const (
	Stderr OutputStream = "stderr"
	Stdout OutputStream = "stdout"
)

type NetworkAccessEnum string

const (
	Enabled    NetworkAccessEnum = "enabled"
	Restricted NetworkAccessEnum = "restricted"
)

type SandboxPolicyType string

const (
	DangerFullAccess SandboxPolicyType = "dangerFullAccess"
	ExternalSandbox  SandboxPolicyType = "externalSandbox"
	ReadOnly         SandboxPolicyType = "readOnly"
	WorkspaceWrite   SandboxPolicyType = "workspaceWrite"
)

type MergeStrategy string

const (
	Replace MergeStrategy = "replace"
	Upsert  MergeStrategy = "upsert"
)

type ApprovalPolicyEnum string

const (
	AskForApprovalUntrusted ApprovalPolicyEnum = "untrusted"
	Never                   ApprovalPolicyEnum = "never"
	OnFailure               ApprovalPolicyEnum = "on-failure"
	OnRequest               ApprovalPolicyEnum = "on-request"
)

// Configures who approval requests are routed to for review. Examples include sandbox
// escapes, blocked network access, MCP approval prompts, and ARC escalations. Defaults to
// `user`. `auto_review` uses a carefully prompted subagent to gather relevant context and
// apply a risk-based decision framework before approving or denying the request. The legacy
// value `guardian_subagent` is accepted for compatibility.
//
// Reviewer currently used for approval requests on this thread.
type ApprovalsReviewer string

const (
	ApprovalsReviewerUser ApprovalsReviewer = "user"
	AutoReview            ApprovalsReviewer = "auto_review"
	GuardianSubagent      ApprovalsReviewer = "guardian_subagent"
)

type ForcedLoginMethod string

const (
	API                      ForcedLoginMethod = "api"
	ForcedLoginMethodChatgpt ForcedLoginMethod = "chatgpt"
)

// Count the full active context against the limit.
//
// Count sampled output and later growth after the carried window prefix.
type AutoCompactTokenLimitScope string

const (
	BodyAfterPrefix AutoCompactTokenLimitScope = "body_after_prefix"
	Total           AutoCompactTokenLimitScope = "total"
)

// See
// https://platform.openai.com/docs/guides/reasoning?api-mode=responses#get-started-with-reasoning
type ReasoningEffort string

const (
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortNone    ReasoningEffort = "none"
	Xhigh                  ReasoningEffort = "xhigh"
)

// Option to disable reasoning summaries.
type ReasoningSummary string

const (
	Auto                 ReasoningSummary = "auto"
	Concise              ReasoningSummary = "concise"
	Detailed             ReasoningSummary = "detailed"
	ReasoningSummaryNone ReasoningSummary = "none"
)

// Controls output length/detail on GPT-5 models via the Responses API. Serialized with
// lowercase values to match the OpenAI API.
type Verbosity string

const (
	VerbosityHigh   Verbosity = "high"
	VerbosityLow    Verbosity = "low"
	VerbosityMedium Verbosity = "medium"
)

type WebSearchMode string

const (
	Cached                WebSearchMode = "cached"
	Live                  WebSearchMode = "live"
	WebSearchModeDisabled WebSearchMode = "disabled"
)

type SandboxMode string

const (
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
)

type ConfigLayerSourceType string

const (
	ConfigLayerSourceTypeMdm          ConfigLayerSourceType = "mdm"
	ConfigLayerSourceTypeProject      ConfigLayerSourceType = "project"
	ConfigLayerSourceTypeSessionFlags ConfigLayerSourceType = "sessionFlags"
	ConfigLayerSourceTypeSystem       ConfigLayerSourceType = "system"
	ConfigLayerSourceTypeUser         ConfigLayerSourceType = "user"
	LegacyManagedConfigTomlFromFile   ConfigLayerSourceType = "legacyManagedConfigTomlFromFile"
	LegacyManagedConfigTomlFromMdm    ConfigLayerSourceType = "legacyManagedConfigTomlFromMdm"
)

type ResidencyRequirement string

const (
	Us ResidencyRequirement = "us"
)

type WriteStatus string

const (
	Ok           WriteStatus = "ok"
	OkOverridden WriteStatus = "okOverridden"
)

type NonSteerableTurnKind string

const (
	NonSteerableTurnKindCompact NonSteerableTurnKind = "compact"
	NonSteerableTurnKindReview  NonSteerableTurnKind = "review"
)

type CodexErrorInfoEnum string

const (
	BadRequest            CodexErrorInfoEnum = "badRequest"
	CodexErrorInfoOther   CodexErrorInfoEnum = "other"
	ContextWindowExceeded CodexErrorInfoEnum = "contextWindowExceeded"
	CyberPolicy           CodexErrorInfoEnum = "cyberPolicy"
	InternalServerError   CodexErrorInfoEnum = "internalServerError"
	SandboxError          CodexErrorInfoEnum = "sandboxError"
	ServerOverloaded      CodexErrorInfoEnum = "serverOverloaded"
	ThreadRollbackFailed  CodexErrorInfoEnum = "threadRollbackFailed"
	Unauthorized          CodexErrorInfoEnum = "unauthorized"
	UsageLimitExceeded    CodexErrorInfoEnum = "usageLimitExceeded"
)

// Lifecycle stage of this feature flag.
//
// Feature is available for user testing and feedback.
//
// Feature is still being built and not ready for broad use.
//
// Feature is production-ready.
//
// Feature is deprecated and should be avoided.
//
// Feature flag is retained only for backwards compatibility.
type ExperimentalFeatureStage string

const (
	Beta             ExperimentalFeatureStage = "beta"
	Deprecated       ExperimentalFeatureStage = "deprecated"
	Removed          ExperimentalFeatureStage = "removed"
	Stable           ExperimentalFeatureStage = "stable"
	UnderDevelopment ExperimentalFeatureStage = "underDevelopment"
)

type ExternalAgentConfigMigrationItemType string

const (
	AgentsMd        ExternalAgentConfigMigrationItemType = "AGENTS_MD"
	Commands        ExternalAgentConfigMigrationItemType = "COMMANDS"
	Config          ExternalAgentConfigMigrationItemType = "CONFIG"
	Hooks           ExternalAgentConfigMigrationItemType = "HOOKS"
	MCPServerConfig ExternalAgentConfigMigrationItemType = "MCP_SERVER_CONFIG"
	Plugins         ExternalAgentConfigMigrationItemType = "PLUGINS"
	Sessions        ExternalAgentConfigMigrationItemType = "SESSIONS"
	Skills          ExternalAgentConfigMigrationItemType = "SKILLS"
	Subagents       ExternalAgentConfigMigrationItemType = "SUBAGENTS"
)

type PatchChangeKindType string

const (
	Add    PatchChangeKindType = "add"
	Delete PatchChangeKindType = "delete"
	Update PatchChangeKindType = "update"
)

type AccountType string

const (
	AccountTypeAPIKey  AccountType = "apiKey"
	AccountTypeChatgpt AccountType = "chatgpt"
	AmazonBedrock      AccountType = "amazonBedrock"
)

type HookOutputEntryKind string

const (
	Context                 HookOutputEntryKind = "context"
	Error                   HookOutputEntryKind = "error"
	Feedback                HookOutputEntryKind = "feedback"
	HookOutputEntryKindStop HookOutputEntryKind = "stop"
	Warning                 HookOutputEntryKind = "warning"
)

type HookEventName string

const (
	HookEventNameStop HookEventName = "stop"
	PermissionRequest HookEventName = "permissionRequest"
	PostCompact       HookEventName = "postCompact"
	PostToolUse       HookEventName = "postToolUse"
	PreCompact        HookEventName = "preCompact"
	PreToolUse        HookEventName = "preToolUse"
	SessionStart      HookEventName = "sessionStart"
	SubagentStart     HookEventName = "subagentStart"
	SubagentStop      HookEventName = "subagentStop"
	UserPromptSubmit  HookEventName = "userPromptSubmit"
)

type HookExecutionMode string

const (
	Async HookExecutionMode = "async"
	Sync  HookExecutionMode = "sync"
)

type HookHandlerType string

const (
	HookHandlerTypeAgent   HookHandlerType = "agent"
	HookHandlerTypeCommand HookHandlerType = "command"
	Prompt                 HookHandlerType = "prompt"
)

type HookScope string

const (
	Thread HookScope = "thread"
	Turn   HookScope = "turn"
)

type HookSource string

const (
	CloudRequirements       HookSource = "cloudRequirements"
	HookSourceMdm           HookSource = "mdm"
	HookSourceProject       HookSource = "project"
	HookSourceSessionFlags  HookSource = "sessionFlags"
	HookSourceSystem        HookSource = "system"
	HookSourceUnknown       HookSource = "unknown"
	HookSourceUser          HookSource = "user"
	LegacyManagedConfigFile HookSource = "legacyManagedConfigFile"
	LegacyManagedConfigMdm  HookSource = "legacyManagedConfigMdm"
	Plugin                  HookSource = "plugin"
)

type HookRunStatus string

const (
	HookRunStatusBlocked   HookRunStatus = "blocked"
	HookRunStatusCompleted HookRunStatus = "completed"
	HookRunStatusFailed    HookRunStatus = "failed"
	HookRunStatusRunning   HookRunStatus = "running"
	Stopped                HookRunStatus = "stopped"
)

type HookTrustStatus string

const (
	HookTrustStatusUntrusted HookTrustStatus = "untrusted"
	Managed                  HookTrustStatus = "managed"
	Modified                 HookTrustStatus = "modified"
	Trusted                  HookTrustStatus = "trusted"
)

type WebSearchActionType string

const (
	FindInPage                WebSearchActionType = "findInPage"
	OpenPage                  WebSearchActionType = "openPage"
	WebSearchActionTypeOther  WebSearchActionType = "other"
	WebSearchActionTypeSearch WebSearchActionType = "search"
)

type CollabAgentStatus string

const (
	CollabAgentStatusCompleted   CollabAgentStatus = "completed"
	CollabAgentStatusErrored     CollabAgentStatus = "errored"
	CollabAgentStatusInterrupted CollabAgentStatus = "interrupted"
	CollabAgentStatusNotFound    CollabAgentStatus = "notFound"
	CollabAgentStatusRunning     CollabAgentStatus = "running"
	PendingInit                  CollabAgentStatus = "pendingInit"
	Shutdown                     CollabAgentStatus = "shutdown"
)

type CommandActionType string

const (
	CommandActionTypeRead    CommandActionType = "read"
	CommandActionTypeSearch  CommandActionType = "search"
	CommandActionTypeUnknown CommandActionType = "unknown"
	ListFiles                CommandActionType = "listFiles"
)

type ImageDetail string

const (
	ImageDetailHigh ImageDetail = "high"
	Original        ImageDetail = "original"
)

type UserInputType string

const (
	LocalImage         UserInputType = "localImage"
	Mention            UserInputType = "mention"
	Skill              UserInputType = "skill"
	UserInputTypeImage UserInputType = "image"
	UserInputTypeText  UserInputType = "text"
)

type InputDynamicToolCallOutputContentItemType string

const (
	InputImage InputDynamicToolCallOutputContentItemType = "inputImage"
	InputText  InputDynamicToolCallOutputContentItemType = "inputText"
)

// Mid-turn assistant text (for example preamble/progress narration).
//
// Additional tool calls or assistant output may follow before turn completion.
//
// The assistant's terminal answer text for the current turn.
type MessagePhase string

const (
	Commentary  MessagePhase = "commentary"
	FinalAnswer MessagePhase = "final_answer"
)

type CommandExecutionSource string

const (
	CommandExecutionSourceAgent CommandExecutionSource = "agent"
	UnifiedExecInteraction      CommandExecutionSource = "unifiedExecInteraction"
	UnifiedExecStartup          CommandExecutionSource = "unifiedExecStartup"
	UserShell                   CommandExecutionSource = "userShell"
)

type ThreadItemType string

const (
	AgentMessage              ThreadItemType = "agentMessage"
	CollabAgentToolCall       ThreadItemType = "collabAgentToolCall"
	CommandExecution          ThreadItemType = "commandExecution"
	ContextCompaction         ThreadItemType = "contextCompaction"
	DynamicToolCall           ThreadItemType = "dynamicToolCall"
	EnteredReviewMode         ThreadItemType = "enteredReviewMode"
	ExitedReviewMode          ThreadItemType = "exitedReviewMode"
	FileChange                ThreadItemType = "fileChange"
	HookPrompt                ThreadItemType = "hookPrompt"
	ImageGeneration           ThreadItemType = "imageGeneration"
	ImageView                 ThreadItemType = "imageView"
	ThreadItemTypeMCPToolCall ThreadItemType = "mcpToolCall"
	ThreadItemTypePlan        ThreadItemType = "plan"
	ThreadItemTypeReasoning   ThreadItemType = "reasoning"
	UserMessage               ThreadItemType = "userMessage"
	WebSearch                 ThreadItemType = "webSearch"
)

type FileSystemAccessMode string

const (
	Deny                     FileSystemAccessMode = "deny"
	FileSystemAccessModeRead FileSystemAccessMode = "read"
	Write                    FileSystemAccessMode = "write"
)

type FileSystemPathType string

const (
	GlobPattern FileSystemPathType = "glob_pattern"
	Path        FileSystemPathType = "path"
	Special     FileSystemPathType = "special"
)

type Kind string

const (
	KindMinimal  Kind = "minimal"
	KindUnknown  Kind = "unknown"
	ProjectRoots Kind = "project_roots"
	Root         Kind = "root"
	SlashTmp     Kind = "slash_tmp"
	Tmpdir       Kind = "tmpdir"
)

type NetworkApprovalProtocol string

const (
	HTTP      NetworkApprovalProtocol = "http"
	HTTPS     NetworkApprovalProtocol = "https"
	Socks5TCP NetworkApprovalProtocol = "socks5Tcp"
	Socks5UDP NetworkApprovalProtocol = "socks5Udp"
)

type GuardianCommandSource string

const (
	Shell       GuardianCommandSource = "shell"
	UnifiedExec GuardianCommandSource = "unifiedExec"
)

type GuardianApprovalReviewActionType string

const (
	ApplyPatch                                  GuardianApprovalReviewActionType = "applyPatch"
	Execve                                      GuardianApprovalReviewActionType = "execve"
	GuardianApprovalReviewActionTypeCommand     GuardianApprovalReviewActionType = "command"
	GuardianApprovalReviewActionTypeMCPToolCall GuardianApprovalReviewActionType = "mcpToolCall"
	NetworkAccess                               GuardianApprovalReviewActionType = "networkAccess"
	RequestPermissions                          GuardianApprovalReviewActionType = "requestPermissions"
)

// [UNSTABLE] Source that produced a terminal approval auto-review decision.
type AutoReviewDecisionSource string

const (
	AutoReviewDecisionSourceAgent AutoReviewDecisionSource = "agent"
)

// [UNSTABLE] Risk level assigned by approval auto-review.
type GuardianRiskLevel string

const (
	Critical                GuardianRiskLevel = "critical"
	GuardianRiskLevelHigh   GuardianRiskLevel = "high"
	GuardianRiskLevelLow    GuardianRiskLevel = "low"
	GuardianRiskLevelMedium GuardianRiskLevel = "medium"
)

// [UNSTABLE] Lifecycle state for an approval auto-review.
type GuardianApprovalReviewStatus string

const (
	Aborted                                GuardianApprovalReviewStatus = "aborted"
	Approved                               GuardianApprovalReviewStatus = "approved"
	Denied                                 GuardianApprovalReviewStatus = "denied"
	GuardianApprovalReviewStatusInProgress GuardianApprovalReviewStatus = "inProgress"
	TimedOut                               GuardianApprovalReviewStatus = "timedOut"
)

// [UNSTABLE] Authorization level assigned by approval auto-review.
type GuardianUserAuthorization string

const (
	GuardianUserAuthorizationHigh    GuardianUserAuthorization = "high"
	GuardianUserAuthorizationLow     GuardianUserAuthorization = "low"
	GuardianUserAuthorizationMedium  GuardianUserAuthorization = "medium"
	GuardianUserAuthorizationUnknown GuardianUserAuthorization = "unknown"
)

type MCPServerStatusDetail string

const (
	MCPServerStatusDetailFull MCPServerStatusDetail = "full"
	ToolsAndAuthOnly          MCPServerStatusDetail = "toolsAndAuthOnly"
)

type MCPAuthStatus string

const (
	BearerToken MCPAuthStatus = "bearerToken"
	NotLoggedIn MCPAuthStatus = "notLoggedIn"
	OAuth       MCPAuthStatus = "oAuth"
	Unsupported MCPAuthStatus = "unsupported"
)

type LoginAccountParamsType string

const (
	ChatgptDeviceCode     LoginAccountParamsType = "chatgptDeviceCode"
	TypeAPIKey            LoginAccountParamsType = "apiKey"
	TypeChatgpt           LoginAccountParamsType = "chatgpt"
	TypeChatgptAuthTokens LoginAccountParamsType = "chatgptAuthTokens"
)

type MCPServerStartupState string

const (
	Cancelled                   MCPServerStartupState = "cancelled"
	MCPServerStartupStateFailed MCPServerStartupState = "failed"
	MCPServerStartupStateReady  MCPServerStartupState = "ready"
	Starting                    MCPServerStartupState = "starting"
)

// Canonical user-input modality tags advertised by a model.
//
// Plain text turns and tool payloads.
//
// Image attachments included in user turns.
type InputModality string

const (
	InputModalityImage InputModality = "image"
	InputModalityText  InputModality = "text"
)

type ModelRerouteReason string

const (
	HighRiskCyberActivity ModelRerouteReason = "highRiskCyberActivity"
)

type ModelVerification string

const (
	TrustedAccessForCyber ModelVerification = "trustedAccessForCyber"
)

type PluginAuthPolicy string

const (
	OnInstall PluginAuthPolicy = "ON_INSTALL"
	OnUse     PluginAuthPolicy = "ON_USE"
)

// Availability state for installing and using the plugin.
//
// Plugin-service currently sends `"ENABLED"` for available remote plugins. Codex app-server
// exposes `"AVAILABLE"` in its API; the alias keeps decoding compatible with that upstream
// response.
type PluginAvailability string

const (
	DisabledByAdmin             PluginAvailability = "DISABLED_BY_ADMIN"
	PluginAvailabilityAVAILABLE PluginAvailability = "AVAILABLE"
)

type PluginInstallPolicy string

const (
	InstalledByDefault           PluginInstallPolicy = "INSTALLED_BY_DEFAULT"
	NotAvailable                 PluginInstallPolicy = "NOT_AVAILABLE"
	PluginInstallPolicyAVAILABLE PluginInstallPolicy = "AVAILABLE"
)

type PluginShareDiscoverability string

const (
	Listed                             PluginShareDiscoverability = "LISTED"
	PluginShareDiscoverabilityPRIVATE  PluginShareDiscoverability = "PRIVATE"
	PluginShareDiscoverabilityUNLISTED PluginShareDiscoverability = "UNLISTED"
)

type PluginSharePrincipalType string

const (
	Group                        PluginSharePrincipalType = "group"
	PluginSharePrincipalTypeUser PluginSharePrincipalType = "user"
	Workspace                    PluginSharePrincipalType = "workspace"
)

type PluginSharePrincipalRole string

const (
	Owner                          PluginSharePrincipalRole = "owner"
	PluginSharePrincipalRoleEditor PluginSharePrincipalRole = "editor"
	PluginSharePrincipalRoleReader PluginSharePrincipalRole = "reader"
)

type PluginSourceType string

const (
	Git                   PluginSourceType = "git"
	PluginSourceTypeLocal PluginSourceType = "local"
	Remote                PluginSourceType = "remote"
)

type PluginListMarketplaceKind string

const (
	PluginListMarketplaceKindLocal PluginListMarketplaceKind = "local"
	SharedWithMe                   PluginListMarketplaceKind = "shared-with-me"
	Vertical                       PluginListMarketplaceKind = "vertical"
	WorkspaceDirectory             PluginListMarketplaceKind = "workspace-directory"
)

type PluginShareTargetRole string

const (
	PluginShareTargetRoleEditor PluginShareTargetRole = "editor"
	PluginShareTargetRoleReader PluginShareTargetRole = "reader"
)

type PluginShareUpdateDiscoverability string

const (
	PluginShareUpdateDiscoverabilityPRIVATE  PluginShareUpdateDiscoverability = "PRIVATE"
	PluginShareUpdateDiscoverabilityUNLISTED PluginShareUpdateDiscoverability = "UNLISTED"
)

type ExecLocalShellActionType string

const (
	ExecLocalShellActionTypeExec       ExecLocalShellActionType = "exec"
	ExecLocalShellActionTypeFindInPage ExecLocalShellActionType = "find_in_page"
	ExecLocalShellActionTypeOpenPage   ExecLocalShellActionType = "open_page"
	ExecLocalShellActionTypeOther      ExecLocalShellActionType = "other"
	ExecLocalShellActionTypeSearch     ExecLocalShellActionType = "search"
)

type ContentType string

const (
	OutputText     ContentType = "output_text"
	ReasoningText  ContentType = "reasoning_text"
	TypeInputImage ContentType = "input_image"
	TypeInputText  ContentType = "input_text"
	TypeText       ContentType = "text"
)

type FunctionCallOutputContentItemType string

const (
	EncryptedContent                            FunctionCallOutputContentItemType = "encrypted_content"
	FunctionCallOutputContentItemTypeInputImage FunctionCallOutputContentItemType = "input_image"
	FunctionCallOutputContentItemTypeInputText  FunctionCallOutputContentItemType = "input_text"
)

type SummaryTextReasoningItemReasoningSummaryType string

const (
	SummaryText SummaryTextReasoningItemReasoningSummaryType = "summary_text"
)

type ResponseItemType string

const (
	Compaction                        ResponseItemType = "compaction"
	CompactionTrigger                 ResponseItemType = "compaction_trigger"
	CustomToolCall                    ResponseItemType = "custom_tool_call"
	CustomToolCallOutput              ResponseItemType = "custom_tool_call_output"
	FunctionCall                      ResponseItemType = "function_call"
	FunctionCallOutput                ResponseItemType = "function_call_output"
	ImageGenerationCall               ResponseItemType = "image_generation_call"
	LocalShellCall                    ResponseItemType = "local_shell_call"
	Message                           ResponseItemType = "message"
	ResponseItemTypeContextCompaction ResponseItemType = "context_compaction"
	ResponseItemTypeOther             ResponseItemType = "other"
	ResponseItemTypeReasoning         ResponseItemType = "reasoning"
	ToolSearchCall                    ResponseItemType = "tool_search_call"
	ToolSearchOutput                  ResponseItemType = "tool_search_output"
	WebSearchCall                     ResponseItemType = "web_search_call"
)

type RemoteControlConnectionStatus string

const (
	Connected                             RemoteControlConnectionStatus = "connected"
	Connecting                            RemoteControlConnectionStatus = "connecting"
	RemoteControlConnectionStatusDisabled RemoteControlConnectionStatus = "disabled"
	RemoteControlConnectionStatusErrored  RemoteControlConnectionStatus = "errored"
)

type ReviewDelivery string

const (
	Detached ReviewDelivery = "detached"
	Inline   ReviewDelivery = "inline"
)

type ReviewTargetType string

const (
	BaseBranch         ReviewTargetType = "baseBranch"
	Commit             ReviewTargetType = "commit"
	Custom             ReviewTargetType = "custom"
	UncommittedChanges ReviewTargetType = "uncommittedChanges"
)

// Describes how much of `items` has been loaded for this turn.
//
// `items` was not loaded for this turn. The field is intentionally empty.
//
// `items` contains only a display summary for this turn.
//
// `items` contains every ThreadItem available from persisted app-server history for this
// turn.
type TurnItemsView string

const (
	Summary                TurnItemsView = "summary"
	TurnItemsViewFull      TurnItemsView = "full"
	TurnItemsViewNotLoaded TurnItemsView = "notLoaded"
)

type TurnStatus string

const (
	TurnStatusCompleted   TurnStatus = "completed"
	TurnStatusFailed      TurnStatus = "failed"
	TurnStatusInProgress  TurnStatus = "inProgress"
	TurnStatusInterrupted TurnStatus = "interrupted"
)

type AddCreditsNudgeCreditType string

const (
	Credits    AddCreditsNudgeCreditType = "credits"
	UsageLimit AddCreditsNudgeCreditType = "usage_limit"
)

type AddCreditsNudgeEmailStatus string

const (
	CooldownActive AddCreditsNudgeEmailStatus = "cooldown_active"
	Sent           AddCreditsNudgeEmailStatus = "sent"
)

type SkillScope string

const (
	Admin            SkillScope = "admin"
	Repo             SkillScope = "repo"
	SkillScopeSystem SkillScope = "system"
	SkillScopeUser   SkillScope = "user"
)

type ThreadSource string

const (
	Subagent                        ThreadSource = "subagent"
	ThreadSourceMemoryConsolidation ThreadSource = "memory_consolidation"
	ThreadSourceUser                ThreadSource = "user"
)

type SubAgentSource string

const (
	SubAgentSourceCompact             SubAgentSource = "compact"
	SubAgentSourceMemoryConsolidation SubAgentSource = "memory_consolidation"
	SubAgentSourceReview              SubAgentSource = "review"
)

type SessionSource string

const (
	SessionSourceAppServer SessionSource = "appServer"
	SessionSourceCLI       SessionSource = "cli"
	SessionSourceExec      SessionSource = "exec"
	SessionSourceUnknown   SessionSource = "unknown"
	SessionSourceVscode    SessionSource = "vscode"
)

type ThreadActiveFlag string

const (
	WaitingOnApproval  ThreadActiveFlag = "waitingOnApproval"
	WaitingOnUserInput ThreadActiveFlag = "waitingOnUserInput"
)

type ThreadStatusType string

const (
	Idle                      ThreadStatusType = "idle"
	SystemError               ThreadStatusType = "systemError"
	ThreadStatusTypeActive    ThreadStatusType = "active"
	ThreadStatusTypeNotLoaded ThreadStatusType = "notLoaded"
)

type ThreadGoalStatus string

const (
	BudgetLimited           ThreadGoalStatus = "budgetLimited"
	Complete                ThreadGoalStatus = "complete"
	Paused                  ThreadGoalStatus = "paused"
	ThreadGoalStatusActive  ThreadGoalStatus = "active"
	ThreadGoalStatusBlocked ThreadGoalStatus = "blocked"
	UsageLimited            ThreadGoalStatus = "usageLimited"
)

type SortDirection string

const (
	Asc  SortDirection = "asc"
	Desc SortDirection = "desc"
)

type ThreadSortKey string

const (
	CreatedAt ThreadSortKey = "created_at"
	UpdatedAt ThreadSortKey = "updated_at"
)

type ThreadSourceKind string

const (
	SubAgent                  ThreadSourceKind = "subAgent"
	SubAgentCompact           ThreadSourceKind = "subAgentCompact"
	SubAgentOther             ThreadSourceKind = "subAgentOther"
	SubAgentReview            ThreadSourceKind = "subAgentReview"
	SubAgentThreadSpawn       ThreadSourceKind = "subAgentThreadSpawn"
	ThreadSourceKindAppServer ThreadSourceKind = "appServer"
	ThreadSourceKindCLI       ThreadSourceKind = "cli"
	ThreadSourceKindExec      ThreadSourceKind = "exec"
	ThreadSourceKindUnknown   ThreadSourceKind = "unknown"
	ThreadSourceKindVscode    ThreadSourceKind = "vscode"
)

type RealtimeConversationVersion string

const (
	V1 RealtimeConversationVersion = "v1"
	V2 RealtimeConversationVersion = "v2"
)

type Personality string

const (
	Friendly        Personality = "friendly"
	PersonalityNone Personality = "none"
	Pragmatic       Personality = "pragmatic"
)

// Initial collaboration mode to use when the TUI starts.
type ModeKind string

const (
	Default      ModeKind = "default"
	ModeKindPlan ModeKind = "plan"
)

type ThreadStartSource string

const (
	Clear   ThreadStartSource = "clear"
	Startup ThreadStartSource = "startup"
)

type ThreadUnsubscribeStatus string

const (
	NotSubscribed                    ThreadUnsubscribeStatus = "notSubscribed"
	ThreadUnsubscribeStatusNotLoaded ThreadUnsubscribeStatus = "notLoaded"
	Unsubscribed                     ThreadUnsubscribeStatus = "unsubscribed"
)

type TurnPlanStepStatus string

const (
	Pending                      TurnPlanStepStatus = "pending"
	TurnPlanStepStatusCompleted  TurnPlanStepStatus = "completed"
	TurnPlanStepStatusInProgress TurnPlanStepStatus = "inProgress"
)

type WindowsSandboxReadiness string

const (
	NotConfigured                WindowsSandboxReadiness = "notConfigured"
	UpdateRequired               WindowsSandboxReadiness = "updateRequired"
	WindowsSandboxReadinessReady WindowsSandboxReadiness = "ready"
)

type WindowsSandboxSetupMode string

const (
	Elevated   WindowsSandboxSetupMode = "elevated"
	Unelevated WindowsSandboxSetupMode = "unelevated"
)

type NetworkAccessUnion struct {
	Bool *bool
	Enum *NetworkAccessEnum
}

func (x *NetworkAccessUnion) UnmarshalJSON(data []byte) error {
	x.Enum = nil
	object, err := unmarshalUnion(data, nil, nil, &x.Bool, nil, false, nil, false, nil, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
	}
	return nil
}

func (x *NetworkAccessUnion) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, x.Bool, nil, false, nil, false, nil, false, nil, x.Enum != nil, x.Enum, false)
}

type ApprovalPolicyUnion struct {
	ApprovalPolicyGranularAskForApproval *ApprovalPolicyGranularAskForApproval
	Enum                                 *ApprovalPolicyEnum
}

func (x *ApprovalPolicyUnion) UnmarshalJSON(data []byte) error {
	x.ApprovalPolicyGranularAskForApproval = nil
	x.Enum = nil
	var c ApprovalPolicyGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.ApprovalPolicyGranularAskForApproval = &c
	}
	return nil
}

func (x *ApprovalPolicyUnion) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.ApprovalPolicyGranularAskForApproval != nil, x.ApprovalPolicyGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, true)
}

// Optional cwd filter or filters; when set, only threads whose session cwd exactly matches
// one of these paths are returned.
type ForcedChatgptWorkspaceIDS struct {
	String      *string
	StringArray []string
}

func (x *ForcedChatgptWorkspaceIDS) UnmarshalJSON(data []byte) error {
	x.StringArray = nil
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, true, &x.StringArray, false, nil, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
	}
	return nil
}

func (x *ForcedChatgptWorkspaceIDS) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, x.StringArray != nil, x.StringArray, false, nil, false, nil, false, nil, true)
}

type AskForApprovalElement struct {
	Enum                         *ApprovalPolicyEnum
	PurpleGranularAskForApproval *PurpleGranularAskForApproval
}

func (x *AskForApprovalElement) UnmarshalJSON(data []byte) error {
	x.PurpleGranularAskForApproval = nil
	x.Enum = nil
	var c PurpleGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.PurpleGranularAskForApproval = &c
	}
	return nil
}

func (x *AskForApprovalElement) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.PurpleGranularAskForApproval != nil, x.PurpleGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, false)
}

type ErrorCodexErrorInfo struct {
	Enum                 *CodexErrorInfoEnum
	PurpleCodexErrorInfo *PurpleCodexErrorInfo
}

func (x *ErrorCodexErrorInfo) UnmarshalJSON(data []byte) error {
	x.PurpleCodexErrorInfo = nil
	x.Enum = nil
	var c PurpleCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.PurpleCodexErrorInfo = &c
	}
	return nil
}

func (x *ErrorCodexErrorInfo) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.PurpleCodexErrorInfo != nil, x.PurpleCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type PurpleContent struct {
	PurpleUserInput *PurpleUserInput
	String          *string
}

func (x *PurpleContent) UnmarshalJSON(data []byte) error {
	x.PurpleUserInput = nil
	var c PurpleUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.PurpleUserInput = &c
	}
	return nil
}

func (x *PurpleContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.PurpleUserInput != nil, x.PurpleUserInput, false, nil, false, nil, false)
}

type PurpleResult struct {
	PurpleMCPToolCallResult *PurpleMCPToolCallResult
	String                  *string
}

func (x *PurpleResult) UnmarshalJSON(data []byte) error {
	x.PurpleMCPToolCallResult = nil
	var c PurpleMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.PurpleMCPToolCallResult = &c
	}
	return nil
}

func (x *PurpleResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.PurpleMCPToolCallResult != nil, x.PurpleMCPToolCallResult, false, nil, false, nil, true)
}

type FluffyContent struct {
	FluffyUserInput *FluffyUserInput
	String          *string
}

func (x *FluffyContent) UnmarshalJSON(data []byte) error {
	x.FluffyUserInput = nil
	var c FluffyUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.FluffyUserInput = &c
	}
	return nil
}

func (x *FluffyContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.FluffyUserInput != nil, x.FluffyUserInput, false, nil, false, nil, false)
}

type FluffyResult struct {
	FluffyMCPToolCallResult *FluffyMCPToolCallResult
	String                  *string
}

func (x *FluffyResult) UnmarshalJSON(data []byte) error {
	x.FluffyMCPToolCallResult = nil
	var c FluffyMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.FluffyMCPToolCallResult = &c
	}
	return nil
}

func (x *FluffyResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.FluffyMCPToolCallResult != nil, x.FluffyMCPToolCallResult, false, nil, false, nil, true)
}

type FunctionCallOutputBody struct {
	FunctionCallOutputContentItemArray []FunctionCallOutputContentItem
	String                             *string
}

func (x *FunctionCallOutputBody) UnmarshalJSON(data []byte) error {
	x.FunctionCallOutputContentItemArray = nil
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, true, &x.FunctionCallOutputContentItemArray, false, nil, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
	}
	return nil
}

func (x *FunctionCallOutputBody) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, x.FunctionCallOutputContentItemArray != nil, x.FunctionCallOutputContentItemArray, false, nil, false, nil, false, nil, false)
}

type CodexErrorInfo2 struct {
	Enum                 *CodexErrorInfoEnum
	FluffyCodexErrorInfo *FluffyCodexErrorInfo
}

func (x *CodexErrorInfo2) UnmarshalJSON(data []byte) error {
	x.FluffyCodexErrorInfo = nil
	x.Enum = nil
	var c FluffyCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.FluffyCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo2) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.FluffyCodexErrorInfo != nil, x.FluffyCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type TentacledContent struct {
	String             *string
	TentacledUserInput *TentacledUserInput
}

func (x *TentacledContent) UnmarshalJSON(data []byte) error {
	x.TentacledUserInput = nil
	var c TentacledUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.TentacledUserInput = &c
	}
	return nil
}

func (x *TentacledContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.TentacledUserInput != nil, x.TentacledUserInput, false, nil, false, nil, false)
}

type TentacledResult struct {
	String                     *string
	TentacledMCPToolCallResult *TentacledMCPToolCallResult
}

func (x *TentacledResult) UnmarshalJSON(data []byte) error {
	x.TentacledMCPToolCallResult = nil
	var c TentacledMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.TentacledMCPToolCallResult = &c
	}
	return nil
}

func (x *TentacledResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.TentacledMCPToolCallResult != nil, x.TentacledMCPToolCallResult, false, nil, false, nil, true)
}

type RequestID struct {
	Integer *int64
	String  *string
}

func (x *RequestID) UnmarshalJSON(data []byte) error {
	object, err := unmarshalUnion(data, &x.Integer, nil, nil, &x.String, false, nil, false, nil, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
	}
	return nil
}

func (x *RequestID) MarshalJSON() ([]byte, error) {
	return marshalUnion(x.Integer, nil, nil, x.String, false, nil, false, nil, false, nil, false, nil, false)
}

type ThreadForkParamsApprovalPolicy struct {
	Enum                         *ApprovalPolicyEnum
	FluffyGranularAskForApproval *FluffyGranularAskForApproval
}

func (x *ThreadForkParamsApprovalPolicy) UnmarshalJSON(data []byte) error {
	x.FluffyGranularAskForApproval = nil
	x.Enum = nil
	var c FluffyGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.FluffyGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadForkParamsApprovalPolicy) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.FluffyGranularAskForApproval != nil, x.FluffyGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, true)
}

type ThreadForkResponseAskForApproval struct {
	Enum                            *ApprovalPolicyEnum
	TentacledGranularAskForApproval *TentacledGranularAskForApproval
}

func (x *ThreadForkResponseAskForApproval) UnmarshalJSON(data []byte) error {
	x.TentacledGranularAskForApproval = nil
	x.Enum = nil
	var c TentacledGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.TentacledGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadForkResponseAskForApproval) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.TentacledGranularAskForApproval != nil, x.TentacledGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, false)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type MagentaSessionSource struct {
	Enum                *SessionSource
	PurpleSessionSource *PurpleSessionSource
}

func (x *MagentaSessionSource) UnmarshalJSON(data []byte) error {
	x.PurpleSessionSource = nil
	x.Enum = nil
	var c PurpleSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.PurpleSessionSource = &c
	}
	return nil
}

func (x *MagentaSessionSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.PurpleSessionSource != nil, x.PurpleSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type MagentaSubAgentSource struct {
	Enum                 *SubAgentSource
	PurpleSubAgentSource *PurpleSubAgentSource
}

func (x *MagentaSubAgentSource) UnmarshalJSON(data []byte) error {
	x.PurpleSubAgentSource = nil
	x.Enum = nil
	var c PurpleSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.PurpleSubAgentSource = &c
	}
	return nil
}

func (x *MagentaSubAgentSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.PurpleSubAgentSource != nil, x.PurpleSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo3 struct {
	Enum                    *CodexErrorInfoEnum
	TentacledCodexErrorInfo *TentacledCodexErrorInfo
}

func (x *CodexErrorInfo3) UnmarshalJSON(data []byte) error {
	x.TentacledCodexErrorInfo = nil
	x.Enum = nil
	var c TentacledCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.TentacledCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo3) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.TentacledCodexErrorInfo != nil, x.TentacledCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type StickyContent struct {
	StickyUserInput *StickyUserInput
	String          *string
}

func (x *StickyContent) UnmarshalJSON(data []byte) error {
	x.StickyUserInput = nil
	var c StickyUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.StickyUserInput = &c
	}
	return nil
}

func (x *StickyContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.StickyUserInput != nil, x.StickyUserInput, false, nil, false, nil, false)
}

type StickyResult struct {
	StickyMCPToolCallResult *StickyMCPToolCallResult
	String                  *string
}

func (x *StickyResult) UnmarshalJSON(data []byte) error {
	x.StickyMCPToolCallResult = nil
	var c StickyMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.StickyMCPToolCallResult = &c
	}
	return nil
}

func (x *StickyResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.StickyMCPToolCallResult != nil, x.StickyMCPToolCallResult, false, nil, false, nil, true)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type DatumSessionSource struct {
	Enum                *SessionSource
	FluffySessionSource *FluffySessionSource
}

func (x *DatumSessionSource) UnmarshalJSON(data []byte) error {
	x.FluffySessionSource = nil
	x.Enum = nil
	var c FluffySessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.FluffySessionSource = &c
	}
	return nil
}

func (x *DatumSessionSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.FluffySessionSource != nil, x.FluffySessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type FriskySubAgentSource struct {
	Enum                 *SubAgentSource
	FluffySubAgentSource *FluffySubAgentSource
}

func (x *FriskySubAgentSource) UnmarshalJSON(data []byte) error {
	x.FluffySubAgentSource = nil
	x.Enum = nil
	var c FluffySubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.FluffySubAgentSource = &c
	}
	return nil
}

func (x *FriskySubAgentSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.FluffySubAgentSource != nil, x.FluffySubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo4 struct {
	Enum                 *CodexErrorInfoEnum
	StickyCodexErrorInfo *StickyCodexErrorInfo
}

func (x *CodexErrorInfo4) UnmarshalJSON(data []byte) error {
	x.StickyCodexErrorInfo = nil
	x.Enum = nil
	var c StickyCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.StickyCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo4) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.StickyCodexErrorInfo != nil, x.StickyCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type IndigoContent struct {
	IndigoUserInput *IndigoUserInput
	String          *string
}

func (x *IndigoContent) UnmarshalJSON(data []byte) error {
	x.IndigoUserInput = nil
	var c IndigoUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.IndigoUserInput = &c
	}
	return nil
}

func (x *IndigoContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.IndigoUserInput != nil, x.IndigoUserInput, false, nil, false, nil, false)
}

type IndigoResult struct {
	IndigoMCPToolCallResult *IndigoMCPToolCallResult
	String                  *string
}

func (x *IndigoResult) UnmarshalJSON(data []byte) error {
	x.IndigoMCPToolCallResult = nil
	var c IndigoMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.IndigoMCPToolCallResult = &c
	}
	return nil
}

func (x *IndigoResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.IndigoMCPToolCallResult != nil, x.IndigoMCPToolCallResult, false, nil, false, nil, true)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type FriskySessionSource struct {
	Enum                   *SessionSource
	TentacledSessionSource *TentacledSessionSource
}

func (x *FriskySessionSource) UnmarshalJSON(data []byte) error {
	x.TentacledSessionSource = nil
	x.Enum = nil
	var c TentacledSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.TentacledSessionSource = &c
	}
	return nil
}

func (x *FriskySessionSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.TentacledSessionSource != nil, x.TentacledSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type MischievousSubAgentSource struct {
	Enum                    *SubAgentSource
	TentacledSubAgentSource *TentacledSubAgentSource
}

func (x *MischievousSubAgentSource) UnmarshalJSON(data []byte) error {
	x.TentacledSubAgentSource = nil
	x.Enum = nil
	var c TentacledSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.TentacledSubAgentSource = &c
	}
	return nil
}

func (x *MischievousSubAgentSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.TentacledSubAgentSource != nil, x.TentacledSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo5 struct {
	Enum                 *CodexErrorInfoEnum
	IndigoCodexErrorInfo *IndigoCodexErrorInfo
}

func (x *CodexErrorInfo5) UnmarshalJSON(data []byte) error {
	x.IndigoCodexErrorInfo = nil
	x.Enum = nil
	var c IndigoCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.IndigoCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo5) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndigoCodexErrorInfo != nil, x.IndigoCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type IndecentContent struct {
	IndecentUserInput *IndecentUserInput
	String            *string
}

func (x *IndecentContent) UnmarshalJSON(data []byte) error {
	x.IndecentUserInput = nil
	var c IndecentUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.IndecentUserInput = &c
	}
	return nil
}

func (x *IndecentContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.IndecentUserInput != nil, x.IndecentUserInput, false, nil, false, nil, false)
}

type IndecentResult struct {
	IndecentMCPToolCallResult *IndecentMCPToolCallResult
	String                    *string
}

func (x *IndecentResult) UnmarshalJSON(data []byte) error {
	x.IndecentMCPToolCallResult = nil
	var c IndecentMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.IndecentMCPToolCallResult = &c
	}
	return nil
}

func (x *IndecentResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.IndecentMCPToolCallResult != nil, x.IndecentMCPToolCallResult, false, nil, false, nil, true)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type MischievousSessionSource struct {
	Enum                *SessionSource
	StickySessionSource *StickySessionSource
}

func (x *MischievousSessionSource) UnmarshalJSON(data []byte) error {
	x.StickySessionSource = nil
	x.Enum = nil
	var c StickySessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.StickySessionSource = &c
	}
	return nil
}

func (x *MischievousSessionSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.StickySessionSource != nil, x.StickySessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type BraggadociousSubAgentSource struct {
	Enum                 *SubAgentSource
	StickySubAgentSource *StickySubAgentSource
}

func (x *BraggadociousSubAgentSource) UnmarshalJSON(data []byte) error {
	x.StickySubAgentSource = nil
	x.Enum = nil
	var c StickySubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.StickySubAgentSource = &c
	}
	return nil
}

func (x *BraggadociousSubAgentSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.StickySubAgentSource != nil, x.StickySubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo6 struct {
	Enum                   *CodexErrorInfoEnum
	IndecentCodexErrorInfo *IndecentCodexErrorInfo
}

func (x *CodexErrorInfo6) UnmarshalJSON(data []byte) error {
	x.IndecentCodexErrorInfo = nil
	x.Enum = nil
	var c IndecentCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.IndecentCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo6) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndecentCodexErrorInfo != nil, x.IndecentCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type HilariousContent struct {
	HilariousUserInput *HilariousUserInput
	String             *string
}

func (x *HilariousContent) UnmarshalJSON(data []byte) error {
	x.HilariousUserInput = nil
	var c HilariousUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.HilariousUserInput = &c
	}
	return nil
}

func (x *HilariousContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.HilariousUserInput != nil, x.HilariousUserInput, false, nil, false, nil, false)
}

type HilariousResult struct {
	HilariousMCPToolCallResult *HilariousMCPToolCallResult
	String                     *string
}

func (x *HilariousResult) UnmarshalJSON(data []byte) error {
	x.HilariousMCPToolCallResult = nil
	var c HilariousMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.HilariousMCPToolCallResult = &c
	}
	return nil
}

func (x *HilariousResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.HilariousMCPToolCallResult != nil, x.HilariousMCPToolCallResult, false, nil, false, nil, true)
}

type ThreadResumeParamsApprovalPolicy struct {
	Enum                         *ApprovalPolicyEnum
	StickyGranularAskForApproval *StickyGranularAskForApproval
}

func (x *ThreadResumeParamsApprovalPolicy) UnmarshalJSON(data []byte) error {
	x.StickyGranularAskForApproval = nil
	x.Enum = nil
	var c StickyGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.StickyGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadResumeParamsApprovalPolicy) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.StickyGranularAskForApproval != nil, x.StickyGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, true)
}

type ThreadResumeResponseAskForApproval struct {
	Enum                         *ApprovalPolicyEnum
	IndigoGranularAskForApproval *IndigoGranularAskForApproval
}

func (x *ThreadResumeResponseAskForApproval) UnmarshalJSON(data []byte) error {
	x.IndigoGranularAskForApproval = nil
	x.Enum = nil
	var c IndigoGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.IndigoGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadResumeResponseAskForApproval) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndigoGranularAskForApproval != nil, x.IndigoGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, false)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type BraggadociousSessionSource struct {
	Enum                *SessionSource
	IndigoSessionSource *IndigoSessionSource
}

func (x *BraggadociousSessionSource) UnmarshalJSON(data []byte) error {
	x.IndigoSessionSource = nil
	x.Enum = nil
	var c IndigoSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.IndigoSessionSource = &c
	}
	return nil
}

func (x *BraggadociousSessionSource) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndigoSessionSource != nil, x.IndigoSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type SubAgentSource1 struct {
	Enum                 *SubAgentSource
	IndigoSubAgentSource *IndigoSubAgentSource
}

func (x *SubAgentSource1) UnmarshalJSON(data []byte) error {
	x.IndigoSubAgentSource = nil
	x.Enum = nil
	var c IndigoSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.IndigoSubAgentSource = &c
	}
	return nil
}

func (x *SubAgentSource1) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndigoSubAgentSource != nil, x.IndigoSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo7 struct {
	Enum                    *CodexErrorInfoEnum
	HilariousCodexErrorInfo *HilariousCodexErrorInfo
}

func (x *CodexErrorInfo7) UnmarshalJSON(data []byte) error {
	x.HilariousCodexErrorInfo = nil
	x.Enum = nil
	var c HilariousCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.HilariousCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo7) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.HilariousCodexErrorInfo != nil, x.HilariousCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type AmbitiousContent struct {
	AmbitiousUserInput *AmbitiousUserInput
	String             *string
}

func (x *AmbitiousContent) UnmarshalJSON(data []byte) error {
	x.AmbitiousUserInput = nil
	var c AmbitiousUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.AmbitiousUserInput = &c
	}
	return nil
}

func (x *AmbitiousContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.AmbitiousUserInput != nil, x.AmbitiousUserInput, false, nil, false, nil, false)
}

type AmbitiousResult struct {
	AmbitiousMCPToolCallResult *AmbitiousMCPToolCallResult
	String                     *string
}

func (x *AmbitiousResult) UnmarshalJSON(data []byte) error {
	x.AmbitiousMCPToolCallResult = nil
	var c AmbitiousMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.AmbitiousMCPToolCallResult = &c
	}
	return nil
}

func (x *AmbitiousResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.AmbitiousMCPToolCallResult != nil, x.AmbitiousMCPToolCallResult, false, nil, false, nil, true)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type SessionSource1 struct {
	Enum                  *SessionSource
	IndecentSessionSource *IndecentSessionSource
}

func (x *SessionSource1) UnmarshalJSON(data []byte) error {
	x.IndecentSessionSource = nil
	x.Enum = nil
	var c IndecentSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.IndecentSessionSource = &c
	}
	return nil
}

func (x *SessionSource1) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndecentSessionSource != nil, x.IndecentSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type SubAgentSource2 struct {
	Enum                   *SubAgentSource
	IndecentSubAgentSource *IndecentSubAgentSource
}

func (x *SubAgentSource2) UnmarshalJSON(data []byte) error {
	x.IndecentSubAgentSource = nil
	x.Enum = nil
	var c IndecentSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.IndecentSubAgentSource = &c
	}
	return nil
}

func (x *SubAgentSource2) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndecentSubAgentSource != nil, x.IndecentSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo8 struct {
	AmbitiousCodexErrorInfo *AmbitiousCodexErrorInfo
	Enum                    *CodexErrorInfoEnum
}

func (x *CodexErrorInfo8) UnmarshalJSON(data []byte) error {
	x.AmbitiousCodexErrorInfo = nil
	x.Enum = nil
	var c AmbitiousCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.AmbitiousCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo8) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.AmbitiousCodexErrorInfo != nil, x.AmbitiousCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type CunningContent struct {
	CunningUserInput *CunningUserInput
	String           *string
}

func (x *CunningContent) UnmarshalJSON(data []byte) error {
	x.CunningUserInput = nil
	var c CunningUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.CunningUserInput = &c
	}
	return nil
}

func (x *CunningContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.CunningUserInput != nil, x.CunningUserInput, false, nil, false, nil, false)
}

type CunningResult struct {
	CunningMCPToolCallResult *CunningMCPToolCallResult
	String                   *string
}

func (x *CunningResult) UnmarshalJSON(data []byte) error {
	x.CunningMCPToolCallResult = nil
	var c CunningMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.CunningMCPToolCallResult = &c
	}
	return nil
}

func (x *CunningResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.CunningMCPToolCallResult != nil, x.CunningMCPToolCallResult, false, nil, false, nil, true)
}

type ThreadSettingsAskForApproval struct {
	Enum                           *ApprovalPolicyEnum
	IndecentGranularAskForApproval *IndecentGranularAskForApproval
}

func (x *ThreadSettingsAskForApproval) UnmarshalJSON(data []byte) error {
	x.IndecentGranularAskForApproval = nil
	x.Enum = nil
	var c IndecentGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.IndecentGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadSettingsAskForApproval) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.IndecentGranularAskForApproval != nil, x.IndecentGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, false)
}

type ThreadStartParamsApprovalPolicy struct {
	Enum                            *ApprovalPolicyEnum
	HilariousGranularAskForApproval *HilariousGranularAskForApproval
}

func (x *ThreadStartParamsApprovalPolicy) UnmarshalJSON(data []byte) error {
	x.HilariousGranularAskForApproval = nil
	x.Enum = nil
	var c HilariousGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.HilariousGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadStartParamsApprovalPolicy) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.HilariousGranularAskForApproval != nil, x.HilariousGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, true)
}

type ThreadStartResponseAskForApproval struct {
	AmbitiousGranularAskForApproval *AmbitiousGranularAskForApproval
	Enum                            *ApprovalPolicyEnum
}

func (x *ThreadStartResponseAskForApproval) UnmarshalJSON(data []byte) error {
	x.AmbitiousGranularAskForApproval = nil
	x.Enum = nil
	var c AmbitiousGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.AmbitiousGranularAskForApproval = &c
	}
	return nil
}

func (x *ThreadStartResponseAskForApproval) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.AmbitiousGranularAskForApproval != nil, x.AmbitiousGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, false)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type SessionSource2 struct {
	Enum                   *SessionSource
	HilariousSessionSource *HilariousSessionSource
}

func (x *SessionSource2) UnmarshalJSON(data []byte) error {
	x.HilariousSessionSource = nil
	x.Enum = nil
	var c HilariousSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.HilariousSessionSource = &c
	}
	return nil
}

func (x *SessionSource2) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.HilariousSessionSource != nil, x.HilariousSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type SubAgentSource3 struct {
	Enum                    *SubAgentSource
	HilariousSubAgentSource *HilariousSubAgentSource
}

func (x *SubAgentSource3) UnmarshalJSON(data []byte) error {
	x.HilariousSubAgentSource = nil
	x.Enum = nil
	var c HilariousSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.HilariousSubAgentSource = &c
	}
	return nil
}

func (x *SubAgentSource3) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.HilariousSubAgentSource != nil, x.HilariousSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo9 struct {
	CunningCodexErrorInfo *CunningCodexErrorInfo
	Enum                  *CodexErrorInfoEnum
}

func (x *CodexErrorInfo9) UnmarshalJSON(data []byte) error {
	x.CunningCodexErrorInfo = nil
	x.Enum = nil
	var c CunningCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.CunningCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo9) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.CunningCodexErrorInfo != nil, x.CunningCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type MagentaContent struct {
	MagentaUserInput *MagentaUserInput
	String           *string
}

func (x *MagentaContent) UnmarshalJSON(data []byte) error {
	x.MagentaUserInput = nil
	var c MagentaUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.MagentaUserInput = &c
	}
	return nil
}

func (x *MagentaContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.MagentaUserInput != nil, x.MagentaUserInput, false, nil, false, nil, false)
}

type MagentaResult struct {
	MagentaMCPToolCallResult *MagentaMCPToolCallResult
	String                   *string
}

func (x *MagentaResult) UnmarshalJSON(data []byte) error {
	x.MagentaMCPToolCallResult = nil
	var c MagentaMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.MagentaMCPToolCallResult = &c
	}
	return nil
}

func (x *MagentaResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.MagentaMCPToolCallResult != nil, x.MagentaMCPToolCallResult, false, nil, false, nil, true)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type SessionSource3 struct {
	AmbitiousSessionSource *AmbitiousSessionSource
	Enum                   *SessionSource
}

func (x *SessionSource3) UnmarshalJSON(data []byte) error {
	x.AmbitiousSessionSource = nil
	x.Enum = nil
	var c AmbitiousSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.AmbitiousSessionSource = &c
	}
	return nil
}

func (x *SessionSource3) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.AmbitiousSessionSource != nil, x.AmbitiousSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type SubAgentSource4 struct {
	AmbitiousSubAgentSource *AmbitiousSubAgentSource
	Enum                    *SubAgentSource
}

func (x *SubAgentSource4) UnmarshalJSON(data []byte) error {
	x.AmbitiousSubAgentSource = nil
	x.Enum = nil
	var c AmbitiousSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.AmbitiousSubAgentSource = &c
	}
	return nil
}

func (x *SubAgentSource4) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.AmbitiousSubAgentSource != nil, x.AmbitiousSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo10 struct {
	Enum                  *CodexErrorInfoEnum
	MagentaCodexErrorInfo *MagentaCodexErrorInfo
}

func (x *CodexErrorInfo10) UnmarshalJSON(data []byte) error {
	x.MagentaCodexErrorInfo = nil
	x.Enum = nil
	var c MagentaCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.MagentaCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo10) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.MagentaCodexErrorInfo != nil, x.MagentaCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type FriskyContent struct {
	FriskyUserInput *FriskyUserInput
	String          *string
}

func (x *FriskyContent) UnmarshalJSON(data []byte) error {
	x.FriskyUserInput = nil
	var c FriskyUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.FriskyUserInput = &c
	}
	return nil
}

func (x *FriskyContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.FriskyUserInput != nil, x.FriskyUserInput, false, nil, false, nil, false)
}

type FriskyResult struct {
	FriskyMCPToolCallResult *FriskyMCPToolCallResult
	String                  *string
}

func (x *FriskyResult) UnmarshalJSON(data []byte) error {
	x.FriskyMCPToolCallResult = nil
	var c FriskyMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.FriskyMCPToolCallResult = &c
	}
	return nil
}

func (x *FriskyResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.FriskyMCPToolCallResult != nil, x.FriskyMCPToolCallResult, false, nil, false, nil, true)
}

// Origin of the thread (CLI, VSCode, codex exec, codex app-server, etc.).
type SessionSource4 struct {
	CunningSessionSource *CunningSessionSource
	Enum                 *SessionSource
}

func (x *SessionSource4) UnmarshalJSON(data []byte) error {
	x.CunningSessionSource = nil
	x.Enum = nil
	var c CunningSessionSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.CunningSessionSource = &c
	}
	return nil
}

func (x *SessionSource4) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.CunningSessionSource != nil, x.CunningSessionSource, false, nil, x.Enum != nil, x.Enum, false)
}

type SubAgentSource5 struct {
	CunningSubAgentSource *CunningSubAgentSource
	Enum                  *SubAgentSource
}

func (x *SubAgentSource5) UnmarshalJSON(data []byte) error {
	x.CunningSubAgentSource = nil
	x.Enum = nil
	var c CunningSubAgentSource
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.CunningSubAgentSource = &c
	}
	return nil
}

func (x *SubAgentSource5) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.CunningSubAgentSource != nil, x.CunningSubAgentSource, false, nil, x.Enum != nil, x.Enum, false)
}

type CodexErrorInfo11 struct {
	Enum                 *CodexErrorInfoEnum
	FriskyCodexErrorInfo *FriskyCodexErrorInfo
}

func (x *CodexErrorInfo11) UnmarshalJSON(data []byte) error {
	x.FriskyCodexErrorInfo = nil
	x.Enum = nil
	var c FriskyCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.FriskyCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo11) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.FriskyCodexErrorInfo != nil, x.FriskyCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type MischievousContent struct {
	MischievousUserInput *MischievousUserInput
	String               *string
}

func (x *MischievousContent) UnmarshalJSON(data []byte) error {
	x.MischievousUserInput = nil
	var c MischievousUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.MischievousUserInput = &c
	}
	return nil
}

func (x *MischievousContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.MischievousUserInput != nil, x.MischievousUserInput, false, nil, false, nil, false)
}

type MischievousResult struct {
	MischievousMCPToolCallResult *MischievousMCPToolCallResult
	String                       *string
}

func (x *MischievousResult) UnmarshalJSON(data []byte) error {
	x.MischievousMCPToolCallResult = nil
	var c MischievousMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.MischievousMCPToolCallResult = &c
	}
	return nil
}

func (x *MischievousResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.MischievousMCPToolCallResult != nil, x.MischievousMCPToolCallResult, false, nil, false, nil, true)
}

type CodexErrorInfo12 struct {
	Enum                      *CodexErrorInfoEnum
	MischievousCodexErrorInfo *MischievousCodexErrorInfo
}

func (x *CodexErrorInfo12) UnmarshalJSON(data []byte) error {
	x.MischievousCodexErrorInfo = nil
	x.Enum = nil
	var c MischievousCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.MischievousCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo12) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.MischievousCodexErrorInfo != nil, x.MischievousCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type BraggadociousContent struct {
	BraggadociousUserInput *BraggadociousUserInput
	String                 *string
}

func (x *BraggadociousContent) UnmarshalJSON(data []byte) error {
	x.BraggadociousUserInput = nil
	var c BraggadociousUserInput
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.BraggadociousUserInput = &c
	}
	return nil
}

func (x *BraggadociousContent) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.BraggadociousUserInput != nil, x.BraggadociousUserInput, false, nil, false, nil, false)
}

type BraggadociousResult struct {
	BraggadociousMCPToolCallResult *BraggadociousMCPToolCallResult
	String                         *string
}

func (x *BraggadociousResult) UnmarshalJSON(data []byte) error {
	x.BraggadociousMCPToolCallResult = nil
	var c BraggadociousMCPToolCallResult
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.BraggadociousMCPToolCallResult = &c
	}
	return nil
}

func (x *BraggadociousResult) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.BraggadociousMCPToolCallResult != nil, x.BraggadociousMCPToolCallResult, false, nil, false, nil, true)
}

// Override the approval policy for this turn and subsequent turns.
type TurnStartParamsApprovalPolicy struct {
	CunningGranularAskForApproval *CunningGranularAskForApproval
	Enum                          *ApprovalPolicyEnum
}

func (x *TurnStartParamsApprovalPolicy) UnmarshalJSON(data []byte) error {
	x.CunningGranularAskForApproval = nil
	x.Enum = nil
	var c CunningGranularAskForApproval
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.CunningGranularAskForApproval = &c
	}
	return nil
}

func (x *TurnStartParamsApprovalPolicy) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.CunningGranularAskForApproval != nil, x.CunningGranularAskForApproval, false, nil, x.Enum != nil, x.Enum, true)
}

type CodexErrorInfo13 struct {
	BraggadociousCodexErrorInfo *BraggadociousCodexErrorInfo
	Enum                        *CodexErrorInfoEnum
}

func (x *CodexErrorInfo13) UnmarshalJSON(data []byte) error {
	x.BraggadociousCodexErrorInfo = nil
	x.Enum = nil
	var c BraggadociousCodexErrorInfo
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.BraggadociousCodexErrorInfo = &c
	}
	return nil
}

func (x *CodexErrorInfo13) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.BraggadociousCodexErrorInfo != nil, x.BraggadociousCodexErrorInfo, false, nil, x.Enum != nil, x.Enum, true)
}

type Content1 struct {
	String     *string
	UserInput1 *UserInput1
}

func (x *Content1) UnmarshalJSON(data []byte) error {
	x.UserInput1 = nil
	var c UserInput1
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.UserInput1 = &c
	}
	return nil
}

func (x *Content1) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.UserInput1 != nil, x.UserInput1, false, nil, false, nil, false)
}

type Result1 struct {
	MCPToolCallResult1 *MCPToolCallResult1
	String             *string
}

func (x *Result1) UnmarshalJSON(data []byte) error {
	x.MCPToolCallResult1 = nil
	var c MCPToolCallResult1
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.MCPToolCallResult1 = &c
	}
	return nil
}

func (x *Result1) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.MCPToolCallResult1 != nil, x.MCPToolCallResult1, false, nil, false, nil, true)
}

type CodexErrorInfo14 struct {
	CodexErrorInfo1 *CodexErrorInfo1
	Enum            *CodexErrorInfoEnum
}

func (x *CodexErrorInfo14) UnmarshalJSON(data []byte) error {
	x.CodexErrorInfo1 = nil
	x.Enum = nil
	var c CodexErrorInfo1
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.CodexErrorInfo1 = &c
	}
	return nil
}

func (x *CodexErrorInfo14) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.CodexErrorInfo1 != nil, x.CodexErrorInfo1, false, nil, x.Enum != nil, x.Enum, true)
}

type Content2 struct {
	String     *string
	UserInput2 *UserInput2
}

func (x *Content2) UnmarshalJSON(data []byte) error {
	x.UserInput2 = nil
	var c UserInput2
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, false)
	if err != nil {
		return err
	}
	if object {
		x.UserInput2 = &c
	}
	return nil
}

func (x *Content2) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.UserInput2 != nil, x.UserInput2, false, nil, false, nil, false)
}

type Result2 struct {
	MCPToolCallResult2 *MCPToolCallResult2
	String             *string
}

func (x *Result2) UnmarshalJSON(data []byte) error {
	x.MCPToolCallResult2 = nil
	var c MCPToolCallResult2
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, false, nil, true, &c, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
		x.MCPToolCallResult2 = &c
	}
	return nil
}

func (x *Result2) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, false, nil, x.MCPToolCallResult2 != nil, x.MCPToolCallResult2, false, nil, false, nil, true)
}

func unmarshalUnion(data []byte, pi **int64, pf **float64, pb **bool, ps **string, haveArray bool, pa interface{}, haveObject bool, pc interface{}, haveMap bool, pm interface{}, haveEnum bool, pe interface{}, nullable bool) (bool, error) {
	if pi != nil {
			*pi = nil
	}
	if pf != nil {
			*pf = nil
	}
	if pb != nil {
			*pb = nil
	}
	if ps != nil {
			*ps = nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
			return false, err
	}

	switch v := tok.(type) {
	case json.Number:
			if pi != nil {
					i, err := v.Int64()
					if err == nil {
							*pi = &i
							return false, nil
					}
			}
			if pf != nil {
					f, err := v.Float64()
					if err == nil {
							*pf = &f
							return false, nil
					}
					return false, errors.New("Unparsable number")
			}
			return false, errors.New("Union does not contain number")
	case float64:
			return false, errors.New("Decoder should not return float64")
	case bool:
			if pb != nil {
					*pb = &v
					return false, nil
			}
			return false, errors.New("Union does not contain bool")
	case string:
			if haveEnum {
					return false, json.Unmarshal(data, pe)
			}
			if ps != nil {
					*ps = &v
					return false, nil
			}
			return false, errors.New("Union does not contain string")
	case nil:
			if nullable {
					return false, nil
			}
			return false, errors.New("Union does not contain null")
	case json.Delim:
			if v == '{' {
					if haveObject {
							return true, json.Unmarshal(data, pc)
					}
					if haveMap {
							return false, json.Unmarshal(data, pm)
					}
					return false, errors.New("Union does not contain object")
			}
			if v == '[' {
					if haveArray {
							return false, json.Unmarshal(data, pa)
					}
					return false, errors.New("Union does not contain array")
			}
			return false, errors.New("Cannot handle delimiter")
	}
	return false, errors.New("Cannot unmarshal union")
}

func marshalUnion(pi *int64, pf *float64, pb *bool, ps *string, haveArray bool, pa interface{}, haveObject bool, pc interface{}, haveMap bool, pm interface{}, haveEnum bool, pe interface{}, nullable bool) ([]byte, error) {
	if pi != nil {
			return json.Marshal(*pi)
	}
	if pf != nil {
			return json.Marshal(*pf)
	}
	if pb != nil {
			return json.Marshal(*pb)
	}
	if ps != nil {
			return json.Marshal(*ps)
	}
	if haveArray {
			return json.Marshal(pa)
	}
	if haveObject {
			return json.Marshal(pc)
	}
	if haveMap {
			return json.Marshal(pm)
	}
	if haveEnum {
			return json.Marshal(pe)
	}
	if nullable {
			return json.Marshal(nil)
	}
	return nil, errors.New("Union must not be null")
}
