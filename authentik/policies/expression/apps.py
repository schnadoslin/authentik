"""Authentik policy_expression app config"""

from django.apps import AppConfig


class AuthentikPolicyExpressionConfig(AppConfig):
    """Authentik policy_expression app config"""

    name = "authentik.policies.expression"
    label = "authentik_policies_expression"
    verbose_name = "authentik Policies.Expression"
