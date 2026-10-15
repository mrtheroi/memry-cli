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
| **Solo si ejecutas `memry usage enable`:** los datos de tokens y de costo de Claude Code: el número de tokens o el costo, el modelo, el tipo de token, el origen de la consulta y la ventana de tiempo de cada uno, y un hash con clave de sus atributos. Todo lo demás que envía Claude Code se descarta al recibirlo | Para graficar tu uso de Claude Code en el panel de memry. |

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
- La CLI de memry no envía telemetría. Las métricas de uso están desactivadas por defecto: memry
  configura Claude Code para enviarlas solo después de que ejecutas `memry usage enable`, y solo al
  servidor de memry en el que iniciaste sesión (más abajo se indica lo que memry puede revisar y lo
  que no).

## Métricas de uso, en detalle

Las métricas de uso están desactivadas salvo que ejecutes `memry usage enable`. Entonces es Claude
Code, no memry, quien decide qué envía, como se documenta en su
[guía de monitoreo](https://code.claude.com/docs/en/monitoring-usage): sus métricas de
OpenTelemetry, como el uso de tokens, el costo, las sesiones y las líneas de código, con atributos
de identidad como tu correo electrónico y el identificador de tu organización, y atributos propios
de cada métrica. Esa lista no es exhaustiva y puede crecer con nuevas versiones de Claude Code.
memry desactiva los atributos opcionales que puede y no activa los registros (logs) ni las trazas
de Claude Code, así que por esta vía no se envían prompts ni código.

**Lo que conserva el servidor de memry es la garantía.** Conserva solo los datos de tokens y de
costo, asociados a tu cuenta de memry: el número de tokens o el costo, el modelo, el tipo de token
(entrada, salida, lectura de caché o creación de caché), el origen de la consulta, la ventana de
tiempo y la hora en que se recibió. Descarta todo lo demás al recibirlo. No guarda tu correo
electrónico, el identificador de tu organización ni ningún otro atributo: solo calcula un hash con
clave (HMAC) del nombre de la métrica y de los atributos de cada dato, para distinguir las series e
ignorar los envíos repetidos. El punto de recepción no registra las solicitudes ni sus encabezados.

**Lo que memry puede revisar.** Antes de activar las métricas de uso, memry revisa el entorno de la
terminal donde se ejecuta `memry usage enable`, tu `settings.json` de Claude Code y los archivos de
configuración administrada de Claude Code, y se niega si allí hay otra telemetría configurada. No
puede ver archivos `--settings` de una sesión, programas auxiliares de políticas administradas,
perfiles de MDM, el registro de Windows, la configuración administrada desde el servidor ni el
entorno desde el que se inicia Claude Code; cualquiera de ellos puede cambiar qué envía Claude Code
o a dónde. Si usas mTLS, Claude Code también puede presentar su certificado de cliente a tu
servidor de memry.

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
