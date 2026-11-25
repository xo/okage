use xmp_writer::*;
use xmp_writer::pdfa::PdfUAIdDescriptionMode;

fn main() {
    // The example of the documentation of the crate.
    let mut writer = XmpWriter::new();
    writer.creator(["Martin Haug"]);
    writer.title([(Some(LangId("de")), "Titel"), (None, "Title")]);
    writer.num_pages(3);
    writer.pdf_keywords("Keyword1, Keyword2");
    writer.description([(None, "Description")]);
    writer.date([DateTime::date(2021, 11, 06)]);
    let mut colors = writer.colorants();
    colors.add_colorant().swatch_name("Red");
    colors.add_colorant().swatch_name("Green");
    drop(colors);
    writer.creator_tool("xmp-writer 0.3.3");
    println!("{}", writer.finish(None));

    // Every property.
    let mut w = XmpWriter::new();
    w.contributor(["a", "b<&>'\""]).coverage("c").date([DateTime::year(2020), DateTime::local_time(2021, 1, 2, 3, 4, 5), DateTime::new(2022, 12, 31, 23, 59, 58, Timezone::Local { hour: -5, minute: 30 }), DateTime::new(2023, 1, 1, 0, 0, 0, Timezone::Utc)])
        .format("application/pdf").identifier("id").language([LangId("en"), LangId("de")]).publisher(["p"]).relation(["r"])
        .rights([(None, "r")]).source("s").subject(["x", "y"]).type_(["t"]);
    w.base_url("http://a?b=1&c=2").create_date(DateTime::date(2000, 1, 1)).xmp_identifier(["i"]).label("l").metadata_date(DateTime::year(1999)).modify_date(DateTime::year(1998)).nickname("n").rating(-1);
    {
        let mut t = w.thumbnails();
        t.add_thumbnail().format_jpeg().width(10).height(20).image("abc");
    }
    w.certificate("c").marked(true).owner(["o"]).usage_terms([(Some(LangId("fr")), "u")]).web_statement("ws");
    {
        let mut d = w.derived_from();
        d.alternate_paths(["p1"]).document_id("d").file_path("f").instance_id("i").last_modify_date(DateTime::year(2001)).manager("m").manager_variant("mv").manage_to("mt").manage_ui("mu").mask_markers(MaskMarkers::All).part_mapping("pm").rendition_class(RenditionClass::Thumbnail { format: Some("JPEG"), size: Some((4, 5)), color_space: Some("RGB") }).rendition_params("rp").to_part("tp").version_id("v");
    }
    w.document_id("doc");
    {
        let mut h = w.history();
        h.add_event().action(ResourceEventAction::Saved).changed("c").instance_id("i").parameters("p").software_agent("s").when(DateTime::year(2002));
        h.add_event().action(ResourceEventAction::Custom("x"));
    }
    {
        let mut i = w.ingredients();
        i.add_ref().mask_markers(MaskMarkers::None);
    }
    w.instance_id("inst");
    w.managed_from().document_id("x");
    w.manager("m").manage_to("mt").manage_ui("mu").manager_variant("mv").original_doc_id("o");
    {
        let mut p = w.pantry();
        p.add_item().instance_id("pi");
    }
    w.rendition_class(RenditionClass::LowResolution).rendition_params("rp").version_id("vi");
    {
        let mut v = w.version_ref();
        let mut ver = v.add_version();
        ver.comments("c").modifier("m").modify_date(DateTime::year(2003)).version("1");
        ver.event().action(ResourceEventAction::Converted);
    }
    {
        let mut j = w.jobs();
        j.add_job().id("1").name("n").url("u");
    }
    {
        let mut c = w.colorants();
        c.add_colorant().type_(ColorantType::Spot).colorant_mode(ColorantMode::Lab).l(1.5).a(-2).b(3).black(0.25).cyan(1.0).magenta(1e21).yellow(0.1).red(1).green(2).blue(3);
    }
    {
        let mut f = w.fonts();
        f.add_font().child_font_files(["a"]).composite(false).font_face("f").font_family("fa").font_file("ff").font_name("fn").font_type(FontType::OpenType).version_string("1");
    }
    w.max_page_size().width(1.25).height(2.0).unit(DimensionUnit::Mm);
    w.num_pages(7).plate_names(["p"]).idq_scheme("s").pdf_keywords("k").pdf_version("1.7").producer("p").trapped(false);
    w.pdfa_part(4).pdfa_amd("a").pdfa_corr("c").pdfa_conformance("F").pdfa_rev(2020).pdfx_version("x").pdfua_part(1).pdfua_amd("a").pdfua_corr("c").pdfua_rev(2024);
    {
        let mut s = w.extension_schemas();
        s.pdfaid(true);
        s.pdfua_id().properties().describe_all(PdfUAIdDescriptionMode::All);
        s.pdf().properties().describe_all();
        s.xmp().properties().describe_all();
        s.xmp_media_management().properties().describe_all();
        s.paged_text().properties().describe_all();
        s.resource_event().properties().describe_changed();
        s.thumbnail().properties().describe_all();
        let mut sch = s.add_schema();
        sch.namespace(Namespace::Custom(Box::new(CustomNamespace::new("Name", "zz", "http://z/"))));
        let mut vt = sch.value_types();
        let mut t = vt.add_value_type();
        t.name("T").namespace(Namespace::XmpJob).description("d");
        t.fields().add_field().name("f").value_type("Text").description("fd");
    }
    w.element("custom", Namespace::Custom(Box::new(CustomNamespace::new("A", "aa", "http://a/")))).value(1.5f32);
    w.element("custom2", Namespace::Custom(Box::new(CustomNamespace::new("A", "ab", "http://a/")))).alternative_array([1i64, 2]);
    println!("{}", w.finish(Some("about")));
}
