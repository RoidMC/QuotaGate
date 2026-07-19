package types

type EventType string

const (
	ActionUserRegister  EventType = "user.register"
	ActionUserLogin     EventType = "user.login"
	ActionUserUpdate    EventType = "user.update"
	ActionUserDelete    EventType = "user.delete"
	ActionUserDisable   EventType = "user.disable"
	ActionUserEnable    EventType = "user.enable"
	ActionUserLock      EventType = "user.lock"
	ActionUserUnlock    EventType = "user.unlock"
	ActionUserSuspend   EventType = "user.suspend"
	ActionUserUnsuspend EventType = "user.unsuspend"
	ActionPasswordReset EventType = "user.password_reset"
	ActionEmailChanged  EventType = "user.email_changed"
	ActionPhoneChanged  EventType = "user.phone_changed"

	ActionVerificationSent   EventType = "user.verification_sent"
	ActionVerificationVerify EventType = "user.verification_verify"
	ActionVerificationFailed EventType = "user.verification_failed"

	ActionSessionCreate    EventType = "session.create"
	ActionSessionRevoke    EventType = "session.revoke"
	ActionSessionRevokeAll EventType = "session.revoke_all"

	ActionMFAEnable  EventType = "mfa.enable"
	ActionMFADisable EventType = "mfa.disable"
	ActionMFAVerify  EventType = "mfa.verify"

	ActionIdentityLink   EventType = "identity.link"
	ActionIdentityUnlink EventType = "identity.unlink"

	ActionOAuthClientCreate EventType = "oauth.client.create"
	ActionOAuthClientUpdate EventType = "oauth.client.update"
	ActionOAuthClientDelete EventType = "oauth.client.delete"

	ActionPolicyCreate EventType = "policy.create"
	ActionPolicyDelete EventType = "policy.delete"
	ActionRoleAssign   EventType = "role.assign"
	ActionRoleRevoke   EventType = "role.revoke"
	ActionRoleChanged  EventType = "role.changed"

	ActionTenantCreate EventType = "tenant.create"
	ActionTenantUpdate EventType = "tenant.update"
	ActionTenantDelete EventType = "tenant.delete"

	EventWebhookTest EventType = "webhook.test"

	// Admin operations - QuotaGate
	ActionChannelCreate  EventType = "channel.create"
	ActionChannelUpdate  EventType = "channel.update"
	ActionChannelDelete  EventType = "channel.delete"
	ActionChannelEnable  EventType = "channel.enable"
	ActionChannelDisable EventType = "channel.disable"

	ActionPlanCreate EventType = "plan.create"
	ActionPlanUpdate EventType = "plan.update"
	ActionPlanDelete EventType = "plan.delete"

	ActionRefundApprove EventType = "billing.refund.approve"
	ActionRefundReject  EventType = "billing.refund.reject"
	ActionInvoiceIssue  EventType = "billing.invoice.issue"
)
