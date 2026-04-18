classDiagram
direction BT
class authors {
   text name
   text description
   text url
   text image_path
   datetime updated
   datetime created
   char(36) id
}
class collection_recipes {
   char(36) collection_id
   char(36) recipe_id
}
class collections {
   char(36) household_id
   char(36) user_id
   text name
   text description
   datetime updated
   datetime created
   char(36) id
}
class equipment {
   text slug
   text name
   text description
   text image_path
   datetime updated
   datetime created
   char(36) id
}
class feed_subscriptions {
   char(36) household_id
   char(36) feed_id
}
class feeds {
   numeric active
   char(36) publisher_id
   text url
   text name
   integer error_count
   datetime last_sync_at
   numeric last_sync_success
   datetime updated
   datetime created
   char(36) id
}
class food {
   text slug
   text name
   text description
   text image_path
   char(36) default_unit_id
   datetime updated
   datetime created
   char(36) id
}
class food_taxonomies {
   char(36) taxonomy_id
   char(36) food_id
}
class households {
   char(36) owner_id
   text name
   datetime updated
   datetime created
   char(36) id
}
class images {
   text entity_type
   char(36) entity_id
   text path
   integer width
   integer height
   text content_type
   integer size
   text caption
   text source_url
   numeric is_default
   integer order
   datetime updated
   datetime created
   char(36) id
}
class meal_plans {
   char(36) household_id
   datetime date
   text meal_type
   char(36) recipe_id
   integer servings
   text description
   datetime updated
   datetime created
   char(36) id
}
class publishers {
   text name
   text description
   text url
   text image_path
   datetime updated
   datetime created
   char(36) id
}
class recipe_equipment {
   char(36) recipe_id
   char(36) equipment_id
}
class recipe_ingredients {
   char(36) recipe_id
   real amount
   real max_amount
   char(36) unit_id
   char(36) food_id
   text name
   text description
   text category
   text raw_text
   datetime updated
   datetime created
   char(36) id
}
class recipe_instructions {
   char(36) recipe_id
   char(36) parent_id
   integer order
   text title
   text text
   text url
   text image_path
   text video_url
   datetime updated
   datetime created
   char(36) id
}
class recipe_nutritions {
   text serving_size
   real calories
   real fats
   real fat_sat
   real fat_trans
   real cholesterol
   real sodium
   real carbs
   real carb_sugar
   real carb_fiber
   real protein
   real salt
   real iron
   real potassium
   real calcium
   real phosphorus
   real magnesium
   real zinc
   real copper
   real selenium
   real manganese
   char(36) recipe_id
}
class recipe_taxonomies {
   char(36) recipe_id
   char(36) taxonomy_id
}
class recipes {
   char(36) parent_id
   char(36) household_id
   char(36) user_id
   text source_url
   text name
   text image_path
   text description
   text language
   char(36) author_id
   char(36) publisher_id
   char(36) feed_id
   text text
   integer prep_time
   integer cook_time
   integer total_time
   text difficulty
   text method
   integer yield
   integer rating_reviews
   integer rating_count
   real rating_value
   text video
   datetime published
   datetime updated
   datetime created
   char(36) id
}
class recipes_saved {
   char(36) household_id
   numeric is_favorite
   datetime updated
   datetime created
   char(36) user_id
   char(36) recipe_id
}
class scheduler_logs {
   text job_type
   char(36) entity_id
   datetime started_at
   datetime completed_at
   text status
   text error_message
   text metadata
   char(36) id
}
class shopping_items {
   char(36) shopping_list_id
   real amount
   text text
   char(36) unit_id
   char(36) food_id
   numeric is_bought
   datetime updated
   datetime created
   char(36) id
}
class shopping_lists {
   char(36) household_id
   text name
   numeric is_default
   datetime updated
   datetime created
   char(36) id
}
class sqlite_master {
   text type
   text name
   text tbl_name
   int rootpage
   text sql
}
class taxonomies {
   text type
   text slug
   text label
   char(36) parent_id
   char(36) canonical_id
   datetime updated
   datetime created
   char(36) id
}
class unit_taxonomies {
   char(36) taxonomy_id
   char(36) unit_id
}
class units {
   text slug
   text name
   datetime updated
   datetime created
   char(36) id
}
class user_tokens {
   char(36) user_id
   text type
   text token
   datetime expires
   datetime created
   char(36) id
}
class users {
   char(36) household_id
   text name
   text email
   numeric email_verified
   text password
   text image_path
   datetime updated
   datetime created
   char(36) id
}

collection_recipes  -->  collections : collection_id:id
collection_recipes  -->  recipes : recipe_id:id
collections  -->  households : household_id:id
collections  -->  users : user_id:id
feed_subscriptions  -->  feeds : feed_id:id
feed_subscriptions  -->  households : household_id:id
feeds  -->  publishers : publisher_id:id
food  -->  units : default_unit_id:id
food_taxonomies  -->  food : food_id:id
food_taxonomies  -->  taxonomies : taxonomy_id:id
meal_plans  -->  households : household_id:id
meal_plans  -->  recipes : recipe_id:id
recipe_equipment  -->  equipment : equipment_id:id
recipe_equipment  -->  recipes : recipe_id:id
recipe_ingredients  -->  food : food_id:id
recipe_ingredients  -->  recipes : recipe_id:id
recipe_ingredients  -->  units : unit_id:id
recipe_instructions  -->  recipe_instructions : parent_id:id
recipe_instructions  -->  recipes : recipe_id:id
recipe_nutritions  -->  recipes : recipe_id:id
recipe_taxonomies  -->  recipes : recipe_id:id
recipe_taxonomies  -->  taxonomies : taxonomy_id:id
recipes  -->  authors : author_id:id
recipes  -->  feeds : feed_id:id
recipes  -->  households : household_id:id
recipes  -->  publishers : publisher_id:id
recipes  -->  recipes : parent_id:id
recipes  -->  users : user_id:id
recipes_saved  -->  households : household_id:id
recipes_saved  -->  recipes : recipe_id:id
recipes_saved  -->  users : user_id:id
shopping_items  -->  food : food_id:id
shopping_items  -->  shopping_lists : shopping_list_id:id
shopping_items  -->  units : unit_id:id
shopping_lists  -->  households : household_id:id
taxonomies  -->  taxonomies : canonical_id:id
taxonomies  -->  taxonomies : parent_id:id
unit_taxonomies  -->  taxonomies : taxonomy_id:id
unit_taxonomies  -->  units : unit_id:id
user_tokens  -->  users : user_id:id
users  -->  households : household_id:id
