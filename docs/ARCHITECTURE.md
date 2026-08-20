# Architecture — `veltylabs/site_manager`

`site_manager` es el **registro de inquilinos** de `misitio.velty.cl`. Este módulo es el dueño soberano del `site_id`.

## Alcance del dominio

Este módulo administra la identidad de los sitios web de los clientes, las membresías de los usuarios sobre cada sitio, las cuotas/límites del plan asignado y las solicitudes de acceso.

- **`Site.Status`**: El zero value es `draft` (no publicado por construcción). Transiciones válidas: `draft -> live`, `live -> suspended`, `suspended -> live`.
- **`SiteMember.Role`**: El zero value es `viewer` (permiso mínimo). Roles: `viewer`, `editor`, `owner`.
- **`MemberOf(userID, siteID)`**: Es la única consulta oficial para responder si un usuario tiene acceso a un sitio y con qué rol.
- **`AccessRequest`**: Registro en el embudo de ventas. Aceptar una solicitud de acceso jamás crea una membresía automáticamente.

---

## Entidades del Dominio

1. **`Site`**: Registro de un sitio web de un cliente de Velty (`slug`, `name`, `domain`, `domain_expires_at`, `plan`, `theme`, `status`, `dirty`, `published_at`, `published_ref`).
2. **`SiteMember`**: Relación entre un usuario (`user_id`) y un sitio (`site_id`) con su rol asignado (`role`).
3. **`Plan`**: Definición de límites y capacidades (`max_pages`, `max_images`, `custom_domain`). Sin lógica de facturación ni precios.
4. **`AccessRequest`**: Solicitudes de acceso iniciadas por prospectos o clientes (`email`, `name`, `message`, `status`, `created_at`).

---

## Tabla de Operaciones (Ops)

| Nombre de Op | Recurso RBAC | Acción RBAC | Descripción |
|---|---|---|---|
| `site_get` | `site` | `read` | Obtiene la información de un sitio por ID |
| `site_create` | `site` | `create` | Crea un nuevo sitio asegurando unicidad de slug |
| `access_request` | Público | - | Registra una solicitud de acceso idempotente por correo |

---

## Ejemplo de Raíz de Composición (Composition Root)

Un servicio o aplicación compone el módulo inyectando la conexión ORM y el generador de IDs:

```go
// db es un *orm.DB inicializado por la app
// ids es una implementación de model.IDGenerator (ej. unixid)

sm, err := sitemanager.New(sitemanager.Deps{
    DB:  db,
    IDs: ids,
})

// Registrar ops en el router/transporte de la app:
sm.MountOps(appRouter)
```
