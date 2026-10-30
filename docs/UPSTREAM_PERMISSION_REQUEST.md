# Черновик обращения автору — не отправлен

Перед отправкой перепроверь наличие LICENSE и существующих обсуждений. Текст ниже — черновик, не подтверждённое разрешение и не поручение автоматически открыть issue.

**Title:** License clarification for amnezia-proxy and a separate Windows GUI client

Hello,

I would like to maintain a public fork of amnezia-proxy and use it as a separately built proxy component for an independent Windows GUI client. The GUI would manage local SOCKS5/HTTP connections and would not intercept system traffic. The two projects would remain in separate repositories, with upstream attribution and a small, reviewable patch set for the component.

I could not find an explicit license in commit f5a7912dba8b128ae5c1a3f68a06be7c1668b908 or its README. Could you clarify the intended license, or add a LICENSE file if you intend to permit reuse?

In particular, we would need permission to modify the component, publish those modifications, and distribute the component binary with the Windows application, including clarity on any attribution, source-availability, or other conditions. We understand that the license of the underlying amneziawg-go dependency does not automatically license this repository.

Until this is clarified, the proposed integration and distribution of a modified component will remain on hold. Thank you.
