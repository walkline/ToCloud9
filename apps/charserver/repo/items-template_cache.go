package repo

import "database/sql"

type ItemsTemplateCache struct {
	cache map[uint32]ItemTemplate
}

func NewItemsTemplateCache(db *sql.DB) (*ItemsTemplateCache, error) {
	cache := map[uint32]ItemTemplate{}
	rows, err := db.Query("SELECT entry, displayid, InventoryType FROM item_template")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		item := ItemTemplate{}
		err = rows.Scan(&item.ID, &item.DisplayID, &item.InventoryType)
		if err != nil {
			return nil, err
		}

		cache[item.ID] = item
	}

	return &ItemsTemplateCache{
		cache: cache,
	}, nil
}

func (i ItemsTemplateCache) TemplateByID(ID uint32) (*ItemTemplate, error) {
	if item, exist := i.cache[ID]; exist {
		return &item, nil
	}
	return nil, ItemsTemplateNotFound
}
