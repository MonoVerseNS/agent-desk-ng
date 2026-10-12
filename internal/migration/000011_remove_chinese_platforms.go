package migration

import (
	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(11, "remove chinese platforms tables and data", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			tx := ctx.Tx

			// 1. Drop orphaned tables
			tables := []string{
				"t_wx_work_kf_conversation",
				"t_wx_work_kf_message_ref",
				"t_wx_work_kf_sync_state",
			}
			for _, table := range tables {
				if err := tx.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
					return err
				}
			}

			// 2. Remove channel rows of removed types
			removedChannelTypes := []string{"wxwork_kf", "wechat_mp", "lark", "wechat"}
			if err := tx.Exec("DELETE FROM t_channel WHERE channel_type IN (?)", removedChannelTypes).Error; err != nil {
				return err
			}

			// 3. Remove outbox entries for removed channels
			if err := tx.Exec("DELETE FROM t_channel_message_outbox WHERE channel_type IN (?)", removedChannelTypes).Error; err != nil {
				return err
			}

			// 4. Remove customer identities of removed external sources
			removedExternalSources := []string{"wxwork_kf", "lark", "wechat"}
			if err := tx.Exec("DELETE FROM t_customer_identity WHERE external_source IN (?)", removedExternalSources).Error; err != nil {
				return err
			}

			// 5. Remove obsolete permissions and role permissions
			removedPermissionCodes := []string{"wxworkOutbox.view"}
			var permissionIDs []int64
			if err := tx.Table("t_permission").Where("code IN (?)", removedPermissionCodes).Pluck("id", &permissionIDs).Error; err == nil && len(permissionIDs) > 0 {
				if err := tx.Exec("DELETE FROM t_role_permission WHERE permission_id IN (?)", permissionIDs).Error; err != nil {
					return err
				}
				if err := tx.Exec("DELETE FROM t_permission WHERE id IN (?)", permissionIDs).Error; err != nil {
					return err
				}
			}

			return nil
		})
	})
}
