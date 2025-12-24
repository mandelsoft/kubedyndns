package v1alpha1

const ValidationConditionType = "Validation"
const ReasonConfigurarationWarning = "ConfigurarationWarning"
const ReasonConfigurarationValid = "ConfigurationValid"

const ReasonDomainNameMissing = "DomainNameMissing"
const ReasonEMailMissing = "EMailMissing"
const ReasonExpireMissing = "ExpireMissing"
const ReasonTTLMissing = "TTLMissing"
const ReasonInvalidFormat = "InvalidFieldFormat"
const ReasonInvalidParent = "InvalidParent"
const ReasonInvalidNesting = "InvalidNesting"
const ReasonInvalidModification = "InvalidModification"

////////////////////////////////////////////////////////////////////////////////

const RuntimeConditionType = "Runtime"

const ReasonUpdateFailed = "UpdateFailed"
const ReasonRuntimeUnavailable = "RuntimeUnavailable"
const ReasonRuntimeDeploying = "RuntimeDeploying"
const ReasonRuntimeAvailable = "RuntimeAvailable"

////////////////////////////////////////////////////////////////////////////////

const ServerConditionType = "DNSServer"
const ReasonServerActive = "HostedZoneActive"
const ReasonServerValidationFailure = "ValidationFailed"

////////////////////////////////////////////////////////////////////////////////

const NameserverConditionType = "Nameserver"

const ReasonNameserverAvailable = "NameserverAvailable"
const ReasonNameserverUnavailable = "NameserverUnavailable"
const ReasonNameserverPending = "NameserverPending"
