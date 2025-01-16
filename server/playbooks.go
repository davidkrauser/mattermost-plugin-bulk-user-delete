package main

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

func purgeCategoriesWithMissingUsers(db *sql.DB) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error when trying to begin transaction: %s", err.Error())
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			err = fmt.Errorf("error when trying to rollback transaction: %s on error: %s",
				rollbackErr.Error(), err.Error())
		}
	}()

	rows, err := tx.Query(`
			SELECT id FROM ir_category
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_category.userid
			  );
		`)
	if err != nil {
		return fmt.Errorf("error finding categories for missing users: %s", err.Error())
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("error parsing category IDs: %s", scanErr.Error())
		}
		ids = append(ids, id)
	}

	categoryItemDeleteQuery := sq.Delete("ir_category_item").
		Where(sq.Eq{"categoryid": ids}).
		PlaceholderFormat(sq.Dollar)

	categoryItemDeleteQueryString, categoryItemDeleteQueryArgs, err := categoryItemDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete category items: %s", err.Error())
	}

	_, err = tx.Exec(categoryItemDeleteQueryString, categoryItemDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook category items: %s", err.Error())
	}

	categoryDeleteQuery := sq.Delete("ir_category").
		Where(sq.Eq{"id": ids}).
		PlaceholderFormat(sq.Dollar)

	categoryDeleteQueryString, categoryDeleteQueryArgs, err := categoryDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook categories: %s", err.Error())
	}

	_, err = tx.Exec(categoryDeleteQueryString, categoryDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook categories: %s", err.Error())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error when trying to commit the transaction: %s", err.Error())
	}

	return nil
}

func purgeDanglingPlaybookMembers(db *sql.DB) error {
	if err := purgeCategoriesWithMissingUsers(db); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_category
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_category.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_playbookautofollow
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_playbookautofollow.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_playbookmember
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_playbookmember.memberid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_run_participants
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_run_participants.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_viewedchannel
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_viewedchannel.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_userinfo
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_userinfo.id
			  );
		`); err != nil {
		return err
	}

	return nil
}

func purgeEmptyPlaybooks(db *sql.DB) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error when trying to begin transaction: %s", err.Error())
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			err = fmt.Errorf("error when trying to rollback transaction: %s on error: %s",
				rollbackErr.Error(), err.Error())
		}
	}()

	rows, err := tx.Query(`
			SELECT id FROM ir_playbook
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_playbookmember
			      WHERE ir_playbookmember.playbookid = ir_playbook.id
			  );
		`)
	if err != nil {
		return fmt.Errorf("error finding playbooks with no members: %s", err.Error())
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("error parsing board IDs: %s", scanErr.Error())
		}
		ids = append(ids, id)
	}

	metricConfigDeleteQuery := sq.Delete("ir_metricconfig").
		Where(sq.Eq{"playbookid": ids}).
		PlaceholderFormat(sq.Dollar)

	metricConfigDeleteQueryString, metricConfigDeleteQueryArgs, err := metricConfigDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook metric configs: %s", err.Error())
	}

	_, err = tx.Exec(metricConfigDeleteQueryString, metricConfigDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook metric configs: %s", err.Error())
	}

	autoFollowDeleteQuery := sq.Delete("ir_playbookautofollow").
		Where(sq.Eq{"playbookid": ids}).
		PlaceholderFormat(sq.Dollar)

	autoFollowDeleteQueryString, autoFollowDeleteQueryArgs, err := autoFollowDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook auto follows: %s", err.Error())
	}

	_, err = tx.Exec(autoFollowDeleteQueryString, autoFollowDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook auto follows: %s", err.Error())
	}

	playbookDeleteQuery := sq.Delete("ir_playbook").
		Where(sq.Eq{"id": ids}).
		PlaceholderFormat(sq.Dollar)

	playbookDeleteQueryString, playbookDeleteQueryArgs, err := playbookDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbooks: %s", err.Error())
	}

	_, err = tx.Exec(playbookDeleteQueryString, playbookDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbooks: %s", err.Error())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error when trying to commit the transaction: %s", err.Error())
	}

	return nil
}

func purgeEmptyPlaybookRuns(db *sql.DB) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error when trying to begin transaction: %s", err.Error())
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			err = fmt.Errorf("error when trying to rollback transaction: %s on error: %s",
				rollbackErr.Error(), err.Error())
		}
	}()

	rows, err := tx.Query(`
			SELECT id FROM ir_incident
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_run_participants
			      WHERE ir_run_participants.incidentid = ir_incident.id
			  );
		`)
	if err != nil {
		return fmt.Errorf("error finding playbook runs with no members: %s", err.Error())
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("error parsing board IDs: %s", scanErr.Error())
		}
		ids = append(ids, id)
	}

	metricDeleteQuery := sq.Delete("ir_metric").
		Where(sq.Eq{"incidentid": ids}).
		PlaceholderFormat(sq.Dollar)

	metricDeleteQueryString, metricDeleteQueryArgs, err := metricDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook run metrics: %s", err.Error())
	}

	_, err = tx.Exec(metricDeleteQueryString, metricDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook metrics: %s", err.Error())
	}

	statusPostsDeleteQuery := sq.Delete("ir_statusposts").
		Where(sq.Eq{"incidentid": ids}).
		PlaceholderFormat(sq.Dollar)

	statusPostsDeleteQueryString, statusPostsDeleteQueryArgs, err := statusPostsDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook run status posts: %s", err.Error())
	}

	_, err = tx.Exec(statusPostsDeleteQueryString, statusPostsDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook run status posts: %s", err.Error())
	}

	timelineEventDeleteQuery := sq.Delete("ir_timelineevent").
		Where(sq.Eq{"incidentid": ids}).
		PlaceholderFormat(sq.Dollar)

	timelineEventDeleteQueryString, timelineEventDeleteQueryArgs, err := timelineEventDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook run timeline events: %s", err.Error())
	}

	_, err = tx.Exec(timelineEventDeleteQueryString, timelineEventDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook run timeline events: %s", err.Error())
	}

	runDeleteQuery := sq.Delete("ir_incident").
		Where(sq.Eq{"id": ids}).
		PlaceholderFormat(sq.Dollar)

	runDeleteQueryString, runDeleteQueryArgs, err := runDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook runs: %s", err.Error())
	}

	_, err = tx.Exec(runDeleteQueryString, runDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook runs: %s", err.Error())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error when trying to commit the transaction: %s", err.Error())
	}

	return nil
}

func purgeDanglingPlaybookData(db *sql.DB) error {
	if _, err := db.Exec(`
			DELETE FROM ir_channelaction
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Channels
			      WHERE Channels.id = ir_channelaction.channelid
			  );
		`); err != nil {
		return err
	}

	return nil
}
