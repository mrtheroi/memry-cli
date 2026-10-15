# Aviso de privacidad de memry

Última actualización: 2026-10-15 · [English version](PRIVACY.md)

memry es un servicio de memoria persistente para agentes de IA de programación. Lo opera Cesar
Valero, con domicilio en México ("nosotros"). Este aviso explica qué datos guarda memry, para qué,
quién más los trata y cómo puedes eliminarlos. memry está en beta pública.

Dudas y solicitudes: **mrtheroi@gmail.com**

## Qué datos guardamos

| Dato | Para qué |
| --- | --- |
| Tu correo electrónico | Para identificar tu cuenta y enviarte los códigos de acceso. |
| Las memorias que guardan tus agentes: título, contenido, tipo, nombre del proyecto, clave de tema e identificador de sesión | Es el servicio: tus agentes las consultan en sesiones posteriores. |
| Los prompts que tus agentes guardan con la herramienta `save-prompt`, con su proyecto e identificador de sesión | Igual que lo anterior. |
| Códigos de acceso, guardados solo como hash | Para iniciar sesión. Vencen a los 5 minutos y se eliminan un día después. |
| Tokens de acceso, guardados solo como hash | Para que tu equipo use el servicio sin volver a iniciar sesión. |
| Tu dirección IP | Para limitar los intentos de inicio de sesión, y en los registros operativos del servidor, que el proveedor de hosting conserva por un tiempo limitado. |
| **Solo si ejecutas `memry usage enable`:** los conteos de tokens y el costo por modelo de Claude Code, con el tipo de token, el modelo, el origen de la consulta, el identificador de sesión y la ventana de tiempo de cada dato | Para graficar tu uso de Claude Code en el panel de memry. No se envían prompts, código ni contenido de archivos. |

Lo que guardan tus agentes depende de ti y de ellos. **No permitas que guarden contraseñas, llaves de
API ni otros secretos** en memry.

En tu equipo, `memry setup` guarda la URL del servidor y tu token de acceso en
`~/.config/memry/config.json`, que solo puede leer tu usuario. Si activas las métricas de uso,
`memry usage enable` también escribe un token de uso, que solo sirve para enviar métricas de uso, en
`~/.claude/settings.json` de Claude Code, y deja ese archivo legible solo para tu usuario.

## Lo que no hacemos

- No vendemos tus datos ni los compartimos con anunciantes.
- No usamos tus memorias para entrenar modelos de IA.
- No leemos tus memorias, salvo que nos pidas ayuda con un problema o que la ley lo exija.
- memry no usa rastreo web, analítica ni cookies.
- La CLI de memry no envía telemetría. Las métricas de uso están desactivadas por defecto: Claude
  Code las envía solo después de que ejecutas `memry usage enable`, y solo al servidor de memry en
  el que iniciaste sesión, nunca a nadie más.

## Quién más trata tus datos

- **Laravel Cloud** aloja el servidor de memry y su base de datos.
- **Resend** entrega los correos con el código de acceso, por lo que recibe tu correo electrónico y
  el código.

Toda la comunicación entre tu equipo y memry usa HTTPS.

## Cuánto tiempo los conservamos

Conservamos tu cuenta, tus memorias, tus prompts y tus métricas de uso hasta que los elimines.
Desactivar las métricas de uso detiene el envío de nuevas; las ya enviadas se eliminan con tu
cuenta. Los códigos de acceso se
eliminan un día después de vencer. Los proveedores de hosting y de correo pueden conservar registros
operativos por un tiempo limitado conforme a sus propias políticas.

## Tus opciones

- **Dejar de usar memry en un equipo:** `memry uninstall` revoca el token de ese equipo y quita
  memry de Claude Code.
- **Dejar de enviar métricas de uso:** `memry usage disable` (o `memry uninstall`) quita el ajuste
  de Claude Code. `memry usage status` muestra si están activadas.
- **Eliminar todo:** `memry delete-account` elimina de forma permanente tu cuenta, todas tus memorias,
  prompts y métricas de uso, tus tokens y tus códigos de acceso. No se puede deshacer.
- **Acceder a tus datos, rectificarlos, cancelarlos u oponerte a su uso (derechos ARCO):** escribe a
  mrtheroi@gmail.com. Responderemos en un plazo máximo de 20 días hábiles.

## Cambios

Si cambiamos este aviso, actualizaremos la fecha de arriba y describiremos el cambio en el
[CHANGELOG](CHANGELOG.md). Si un cambio afecta el uso que damos a tus datos, te avisaremos por correo
antes de que entre en vigor.
