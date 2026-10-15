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
| **Solo si ejecutas `memry usage enable`:** los datos de tokens y de costo de Claude Code: el número de tokens o el costo, el modelo, el tipo de token, el origen de la consulta y la ventana de tiempo de cada uno, y un hash de todos sus atributos | Para graficar tu uso de Claude Code en el panel de memry. |

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

## Métricas de uso, en detalle

Claude Code no permite filtrar métricas. Al activar las métricas de uso, envía sus 8 métricas de
OpenTelemetry al servidor de memry: número de sesiones, líneas de código, pull requests, commits,
costo, uso de tokens, decisiones de la herramienta de edición de código y tiempo activo. Cada una
lleva tu correo electrónico, el identificador de tu organización, un identificador de usuario
anónimo y el tipo de terminal, que Claude Code no permite omitir. memry desactiva el identificador
de sesión y el UUID de la cuenta. El bloque de recurso de OTLP también indica el servicio y su
versión, el tipo y la versión del sistema operativo y la arquitectura del equipo; memry no activa
las métricas de uso mientras `OTEL_RESOURCE_ATTRIBUTES` agregue atributos personalizados, haya un
generador de encabezados (`otelHeadersHelper`) o la configuración administrada de Claude Code
defina telemetría. Las políticas que memry no puede ver, como la configuración administrada desde
el servidor o un perfil de MDM, pueden cambiar a dónde se envían las métricas. No se envían prompts ni código: memry
no activa los registros (logs) ni las trazas de Claude Code. Antes de activar las métricas de uso,
memry revisa la configuración de OpenTelemetry de la terminal donde se ejecuta
`memry usage enable`; si Claude Code se inicia desde otro entorno que define variables de
OpenTelemetry, estas tienen prioridad sobre lo que memry configuró.

El servidor de memry conserva solo los datos de tokens y de costo, asociados a tu cuenta de memry:
el número de tokens o el costo, el modelo, el tipo de token (entrada, salida, lectura de caché o
creación de caché), el origen de la consulta, la ventana de tiempo y la hora en que se recibió.
Descarta las otras 6 métricas al recibirlas. No guarda tu correo electrónico, el identificador de
tu organización, el identificador de usuario, el tipo de terminal ni ningún otro atributo: solo
calcula un hash SHA-256 del nombre de la métrica y de todos los atributos de cada dato, y guarda
ese hash para distinguir las series e ignorar los envíos repetidos.

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
